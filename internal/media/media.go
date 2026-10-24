// Package media — загрузка файлов и производные размеры изображений.
package media

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif" // регистрирует декодер GIF для image.Decode
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/image/draw"

	"github.com/ykkssyaa/vestnik-cms/internal/access"
	"github.com/ykkssyaa/vestnik-cms/internal/store"
)

type Media = store.Media

// MaxSize — предельный размер загружаемого файла.
const MaxSize = 10 << 20

var (
	ErrTooLarge    = errors.New("файл больше 10 МБ")
	ErrUnsupported = errors.New("поддерживаются только JPEG, PNG и GIF")
	ErrForbidden   = errors.New("недостаточно прав")
)

// Spec — правило построения производного размера.
type Spec struct {
	Name   string
	Width  int
	Height int // 0 — пропорционально; иначе обрезка по центру до Width×Height
}

// Variants — производные размеры из контент-модели (раздел 6).
var Variants = []Spec{
	{Name: "thumb", Width: 320, Height: 320},
	{Name: "medium", Width: 800},
	{Name: "large", Width: 1600},
}

var formats = map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/gif": "gif"}

type Service struct {
	DB   *sql.DB
	Dir  string // корень каталога загрузок
	Auth access.Authorizer
	Now  func() time.Time
}

func NewService(db *sql.DB, dir string, auth access.Authorizer) *Service {
	return &Service{DB: db, Dir: dir, Auth: auth, Now: func() time.Time { return time.Now().UTC() }}
}

// Upload проверяет файл, сохраняет оригинал и производные размеры.
func (s *Service) Upload(ctx context.Context, actor access.Actor, filename string, r io.Reader) (*Media, error) {
	if !s.Auth.Can(actor, "media.upload", access.Resource{}) {
		return nil, ErrForbidden
	}
	raw, err := io.ReadAll(io.LimitReader(r, MaxSize+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxSize {
		return nil, ErrTooLarge
	}
	// Тип определяем по содержимому, а не по расширению имени: имя присылает клиент.
	mime := http.DetectContentType(raw)
	ext, ok := formats[mime]
	if !ok {
		return nil, ErrUnsupported
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnsupported, err)
	}

	now := s.Now()
	base := path.Join(now.Format("2006"), now.Format("01"), randomName())
	m := &Media{
		Filename: cleanFilename(filename), Path: base + "." + ext, MIME: mime, Size: int64(len(raw)),
		Width: img.Bounds().Dx(), Height: img.Bounds().Dy(), UploadedBy: actor.ID, CreatedAt: now,
		Variants: map[string]store.Variant{},
	}
	if err := s.write(m.Path, raw); err != nil {
		return nil, err
	}
	written := []string{m.Path}
	cleanup := func() {
		for _, p := range written {
			os.Remove(s.abs(p))
		}
	}
	for _, spec := range Variants {
		out := Resize(img, spec)
		vext := "png"
		if ext == "jpg" {
			vext = "jpg"
		}
		v := store.Variant{Name: spec.Name, Path: base + "-" + spec.Name + "." + vext,
			Width: out.Bounds().Dx(), Height: out.Bounds().Dy()}
		var buf bytes.Buffer
		if vext == "jpg" {
			err = jpeg.Encode(&buf, out, &jpeg.Options{Quality: 85})
		} else {
			err = png.Encode(&buf, out)
		}
		if err == nil {
			err = s.write(v.Path, buf.Bytes())
		}
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("variant %s: %w", spec.Name, err)
		}
		written = append(written, v.Path)
		m.Variants[spec.Name] = v
	}
	if err := store.NewMediaRepo(s.DB).Insert(ctx, m); err != nil {
		cleanup()
		return nil, err
	}
	return m, nil
}

// Resize строит производное изображение. Изображение никогда не увеличивается.
func Resize(src image.Image, spec Spec) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if spec.Height == 0 {
		tw := min(spec.Width, w)
		th := max(1, h*tw/w)
		dst := image.NewRGBA(image.Rect(0, 0, tw, th))
		draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
		return dst
	}
	// Обрезка по центру до пропорций спецификации, затем уменьшение.
	targetRatio := float64(spec.Width) / float64(spec.Height)
	cw, ch := w, h
	if float64(w)/float64(h) > targetRatio {
		cw = int(float64(h) * targetRatio)
	} else {
		ch = int(float64(w) / targetRatio)
	}
	crop := image.Rect(b.Min.X+(w-cw)/2, b.Min.Y+(h-ch)/2, b.Min.X+(w-cw)/2+cw, b.Min.Y+(h-ch)/2+ch)
	tw, th := min(spec.Width, cw), min(spec.Height, ch)
	dst := image.NewRGBA(image.Rect(0, 0, tw, th))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, crop, draw.Src, nil)
	return dst
}

func (s *Service) abs(rel string) string { return filepath.Join(s.Dir, filepath.FromSlash(rel)) }

func (s *Service) write(rel string, data []byte) error {
	p := s.abs(rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

func randomName() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// cleanFilename оставляет от присланного имени только базовое имя для показа.
func cleanFilename(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, `\`, "/"))
	if !utf8.ValidString(name) || name == "." || name == "/" {
		return "file"
	}
	if len(name) > 200 {
		name = name[:200]
	}
	return name
}

func (s *Service) Get(ctx context.Context, id int64) (*Media, error) {
	return store.NewMediaRepo(s.DB).Get(ctx, id)
}

func (s *Service) List(ctx context.Context, limit, offset int) ([]*Media, int, error) {
	return store.NewMediaRepo(s.DB).List(ctx, limit, offset)
}

func (s *Service) SetAlt(ctx context.Context, actor access.Actor, id int64, alt string) error {
	if !s.Auth.Can(actor, "media.upload", access.Resource{}) {
		return ErrForbidden
	}
	return store.NewMediaRepo(s.DB).UpdateAlt(ctx, id, strings.TrimSpace(alt))
}

// Delete удаляет записи и файлы оригинала и производных.
func (s *Service) Delete(ctx context.Context, actor access.Actor, id int64) error {
	if !s.Auth.Can(actor, "media.delete", access.Resource{}) {
		return ErrForbidden
	}
	repo := store.NewMediaRepo(s.DB)
	m, err := repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := repo.Delete(ctx, id); err != nil {
		return err
	}
	os.Remove(s.abs(m.Path))
	for _, v := range m.Variants {
		os.Remove(s.abs(v.Path))
	}
	return nil
}
