package media

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ykkssyaa/vestnik-cms/internal/access"
	"github.com/ykkssyaa/vestnik-cms/internal/store"
)

func newService(t *testing.T) (*Service, access.Actor) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	u := &store.User{Email: "e@x", Name: "Ред", Role: "editor", PasswordHash: "-", Active: true}
	store.NewUserRepo(db).Insert(context.Background(), u)
	return NewService(db, t.TempDir(), access.AllowAll{}), access.Actor{ID: u.ID, Role: u.Role}
}

func testImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}
	return img
}

func encodeJPEG(t *testing.T, img image.Image) []byte {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestUploadBuildsVariants(t *testing.T) {
	svc, actor := newService(t)
	m, err := svc.Upload(context.Background(), actor, "../../etc/фото.jpg", bytes.NewReader(encodeJPEG(t, testImage(2000, 1000))))
	if err != nil {
		t.Fatal(err)
	}
	if m.Filename != "фото.jpg" {
		t.Errorf("имя файла не очищено от пути: %q", m.Filename)
	}
	if m.Width != 2000 || m.Height != 1000 || m.MIME != "image/jpeg" {
		t.Errorf("метаданные: %+v", m)
	}
	want := map[string][2]int{"thumb": {320, 320}, "medium": {800, 400}, "large": {1600, 800}}
	for name, wh := range want {
		v, ok := m.Variants[name]
		if !ok {
			t.Fatalf("нет варианта %s", name)
		}
		if v.Width != wh[0] || v.Height != wh[1] {
			t.Errorf("%s: %dx%d, ожидали %dx%d", name, v.Width, v.Height, wh[0], wh[1])
		}
		if _, err := os.Stat(filepath.Join(svc.Dir, v.Path)); err != nil {
			t.Errorf("%s: файл не записан: %v", name, err)
		}
	}
	stored, err := svc.Get(context.Background(), m.ID)
	if err != nil || len(stored.Variants) != 3 {
		t.Fatalf("варианты не сохранены в БД: %v %+v", err, stored)
	}
}

func TestSmallImageIsNotUpscaled(t *testing.T) {
	cases := []struct {
		spec       Spec
		srcW, srcH int
		wantW      int
		wantH      int
	}{
		{Spec{Name: "large", Width: 1600}, 500, 300, 500, 300},
		{Spec{Name: "medium", Width: 800}, 1000, 500, 800, 400},
		{Spec{Name: "thumb", Width: 320, Height: 320}, 200, 100, 100, 100},
		{Spec{Name: "thumb", Width: 320, Height: 320}, 400, 1200, 320, 320},
	}
	for _, c := range cases {
		out := Resize(testImage(c.srcW, c.srcH), c.spec)
		if out.Bounds().Dx() != c.wantW || out.Bounds().Dy() != c.wantH {
			t.Errorf("%s %dx%d -> %dx%d, ожидали %dx%d", c.spec.Name, c.srcW, c.srcH,
				out.Bounds().Dx(), out.Bounds().Dy(), c.wantW, c.wantH)
		}
	}
}

func TestPNGKeepsPNGVariants(t *testing.T) {
	svc, actor := newService(t)
	var buf bytes.Buffer
	png.Encode(&buf, testImage(100, 100))
	m, err := svc.Upload(context.Background(), actor, "a.png", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(m.Variants["thumb"].Path, ".png") {
		t.Fatalf("прозрачность PNG потеряется в JPEG: %s", m.Variants["thumb"].Path)
	}
}

func TestUploadRejectsNonImages(t *testing.T) {
	svc, actor := newService(t)
	cases := map[string][]byte{
		"html with jpg name": []byte("<html><script>alert(1)</script></html>"),
		"pdf":                []byte("%PDF-1.4 ..."),
		"broken jpeg":        append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 100)...),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.Upload(context.Background(), actor, "x.jpg", bytes.NewReader(body)); !errors.Is(err, ErrUnsupported) {
				t.Fatalf("ожидали ErrUnsupported, получили %v", err)
			}
		})
	}
}

func TestUploadRejectsTooLarge(t *testing.T) {
	svc, actor := newService(t)
	big := bytes.NewReader(make([]byte, MaxSize+10))
	if _, err := svc.Upload(context.Background(), actor, "big.jpg", big); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("ожидали ErrTooLarge, получили %v", err)
	}
}

func TestDeleteRemovesFiles(t *testing.T) {
	svc, actor := newService(t)
	m, err := svc.Upload(context.Background(), actor, "a.jpg", bytes.NewReader(encodeJPEG(t, testImage(50, 50))))
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(context.Background(), actor, m.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(svc.Dir, m.Path)); !os.IsNotExist(err) {
		t.Fatal("оригинал не удалён")
	}
	if _, err := svc.Get(context.Background(), m.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("запись не удалена")
	}
}
