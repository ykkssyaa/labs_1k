package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ykkssyaa/vestnik-cms/internal/access"
	"github.com/ykkssyaa/vestnik-cms/internal/store"
)

type Entry = store.Entry
type Revision = store.Revision

// Статусы жизненного цикла.
const (
	StatusDraft     = "draft"
	StatusReview    = "review"
	StatusPublished = "published"
	StatusArchived  = "archived"
)

var (
	ErrNotFound    = store.ErrNotFound
	ErrForbidden   = errors.New("недостаточно прав")
	ErrUnknownType = errors.New("неизвестный тип материала")
)

// AuditSink получает события ядра. Пишет в той же транзакции, что и изменение.
type AuditSink interface {
	Record(ctx context.Context, q store.DBTX, actor access.Actor, action, object string, objectID int64, details string) error
}

// Input — то, что пользователь присылает при создании или правке материала.
type Input struct {
	Title          string         `json:"title"`
	Slug           string         `json:"slug"`
	SEOTitle       string         `json:"seo_title"`
	SEODescription string         `json:"seo_description"`
	Data           map[string]any `json:"data"`
	CategoryIDs    []int64        `json:"category_ids"`
	Tags           []string       `json:"tags"`
}

// Snapshot — содержимое материала в ревизии.
type Snapshot struct {
	Title          string         `json:"title"`
	Slug           string         `json:"slug"`
	SEOTitle       string         `json:"seo_title"`
	SEODescription string         `json:"seo_description"`
	Data           map[string]any `json:"data"`
	CategoryIDs    []int64        `json:"category_ids"`
	TagIDs         []int64        `json:"tag_ids"`
}

type Service struct {
	DB    *sql.DB
	Types *Registry
	Auth  access.Authorizer
	Audit AuditSink // nil — не журналировать
	Now   func() time.Time
}

func NewService(db *sql.DB, types *Registry, auth access.Authorizer) *Service {
	return &Service{DB: db, Types: types, Auth: auth, Now: func() time.Time { return time.Now().UTC().Truncate(time.Second) }}
}

func resourceOf(e *Entry) access.Resource {
	return access.Resource{Type: e.Type, AuthorID: e.AuthorID, Status: e.Status}
}

func (s *Service) can(actor access.Actor, action string, res access.Resource) error {
	if !s.Auth.Can(actor, action, res) {
		return fmt.Errorf("%w: %s", ErrForbidden, action)
	}
	return nil
}

func (s *Service) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Service) audit(ctx context.Context, q store.DBTX, actor access.Actor, action string, e *Entry, details string) error {
	if s.Audit == nil {
		return nil
	}
	return s.Audit.Record(ctx, q, actor, action, "entry", e.ID, details)
}

// checkCommon проверяет общие поля материала и нормализует slug.
func checkCommon(in *Input) FieldErrors {
	errs := FieldErrors{}
	in.Title = strings.TrimSpace(in.Title)
	in.Slug = strings.TrimSpace(strings.ToLower(in.Slug))
	in.SEOTitle = strings.TrimSpace(in.SEOTitle)
	in.SEODescription = strings.TrimSpace(in.SEODescription)
	if n := utf8.RuneCountInString(in.Title); n == 0 || n > 200 {
		errs["title"] = "заголовок обязателен, до 200 символов"
	}
	if in.Slug == "" {
		in.Slug = Slugify(in.Title)
	}
	if !ValidSlug(in.Slug) {
		errs["slug"] = "только латиница в нижнем регистре, цифры и дефисы"
	}
	if utf8.RuneCountInString(in.SEOTitle) > 70 {
		errs["seo_title"] = "не длиннее 70 символов"
	}
	if utf8.RuneCountInString(in.SEODescription) > 160 {
		errs["seo_description"] = "не длиннее 160 символов"
	}
	return errs
}

// prepare валидирует ввод по схеме типа и заполняет содержимое материала.
func (s *Service) prepare(typ *Type, in Input, e *Entry) error {
	errs := checkCommon(&in)
	data, dataErrs := typ.Validate(in.Data)
	for k, v := range dataErrs {
		errs["data."+k] = v
	}
	if len(errs) > 0 {
		return errs
	}
	e.Title, e.Slug, e.SEOTitle, e.SEODescription, e.Data = in.Title, in.Slug, in.SEOTitle, in.SEODescription, data
	e.Search = searchText(typ, e)
	return nil
}

// searchText — строка для поиска: заголовок и текстовые поля в нижнем регистре.
func searchText(typ *Type, e *Entry) string {
	parts := []string{e.Title, e.SEODescription}
	for _, f := range typ.Fields {
		if v, ok := e.Data[f.Name].(string); ok && f.Type != FieldURL {
			parts = append(parts, v)
		}
	}
	return strings.ToLower(strings.Join(parts, " "))
}

// uniqueSlug добавляет -2, -3… если slug занят в пределах типа.
func uniqueSlug(ctx context.Context, repo *store.EntryRepo, e *Entry) error {
	base := e.Slug
	for i := 2; ; i++ {
		taken, err := repo.SlugTaken(ctx, e.Type, e.Slug, e.ID)
		if err != nil {
			return err
		}
		if !taken {
			return nil
		}
		e.Slug = fmt.Sprintf("%s-%d", base, i)
	}
}

func (s *Service) typeOf(name string) (*Type, error) {
	t, ok := s.Types.Get(name)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownType, name)
	}
	return t, nil
}

// Create создаёт черновик и первую ревизию.
func (s *Service) Create(ctx context.Context, actor access.Actor, typeName string, in Input) (*Entry, error) {
	typ, err := s.typeOf(typeName)
	if err != nil {
		return nil, err
	}
	if err := s.can(actor, "entry.create", access.Resource{Type: typ.Name}); err != nil {
		return nil, err
	}
	now := s.Now()
	e := &Entry{Type: typ.Name, Status: StatusDraft, AuthorID: actor.ID, CreatedAt: now, UpdatedAt: now}
	if err := s.prepare(typ, in, e); err != nil {
		return nil, err
	}
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		repo := store.NewEntryRepo(tx)
		if err := uniqueSlug(ctx, repo, e); err != nil {
			return err
		}
		if err := repo.Insert(ctx, e); err != nil {
			return err
		}
		if err := s.saveTerms(ctx, tx, e, in); err != nil {
			return err
		}
		if err := s.addRevision(ctx, tx, actor, e, "создание"); err != nil {
			return err
		}
		return s.audit(ctx, tx, actor, "entry.create", e, e.Title)
	})
	if err != nil {
		return nil, err
	}
	return e, nil
}

// Update сохраняет новое содержимое материала и ревизию.
func (s *Service) Update(ctx context.Context, actor access.Actor, id int64, in Input) (*Entry, error) {
	var e *Entry
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		repo := store.NewEntryRepo(tx)
		var err error
		if e, err = repo.Get(ctx, id); err != nil {
			return err
		}
		if err := s.can(actor, "entry.edit", resourceOf(e)); err != nil {
			return err
		}
		typ, err := s.typeOf(e.Type)
		if err != nil {
			return err
		}
		if err := s.prepare(typ, in, e); err != nil {
			return err
		}
		e.UpdatedAt = s.Now()
		if err := uniqueSlug(ctx, repo, e); err != nil {
			return err
		}
		if err := repo.Update(ctx, e); err != nil {
			return err
		}
		if err := s.saveTerms(ctx, tx, e, in); err != nil {
			return err
		}
		if err := s.addRevision(ctx, tx, actor, e, "правка"); err != nil {
			return err
		}
		return s.audit(ctx, tx, actor, "entry.update", e, e.Title)
	})
	if err != nil {
		return nil, err
	}
	return e, nil
}

// Delete удаляет материал вместе с ревизиями и связями (каскад в БД).
func (s *Service) Delete(ctx context.Context, actor access.Actor, id int64) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		repo := store.NewEntryRepo(tx)
		e, err := repo.Get(ctx, id)
		if err != nil {
			return err
		}
		if err := s.can(actor, "entry.delete", resourceOf(e)); err != nil {
			return err
		}
		if err := repo.Delete(ctx, id); err != nil {
			return err
		}
		return s.audit(ctx, tx, actor, "entry.delete", e, e.Title)
	})
}

// Get возвращает материал с рубриками и тегами; данные приведены к типам схемы.
func (s *Service) Get(ctx context.Context, id int64) (*Entry, error) {
	e, err := store.NewEntryRepo(s.DB).Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return e, s.hydrate(ctx, e)
}

func (s *Service) GetBySlug(ctx context.Context, typeName, slug string) (*Entry, error) {
	e, err := store.NewEntryRepo(s.DB).GetBySlug(ctx, typeName, slug)
	if err != nil {
		return nil, err
	}
	return e, s.hydrate(ctx, e)
}

// List — список материалов по фильтру с рубриками и тегами.
func (s *Service) List(ctx context.Context, f store.EntryFilter) ([]*Entry, int, error) {
	list, total, err := store.NewEntryRepo(s.DB).List(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	for _, e := range list {
		if err := s.hydrate(ctx, e); err != nil {
			return nil, 0, err
		}
	}
	return list, total, nil
}

func (s *Service) hydrate(ctx context.Context, e *Entry) error {
	if typ, ok := s.Types.Get(e.Type); ok {
		e.Data = typ.Normalize(e.Data)
	}
	return s.loadTerms(ctx, e)
}

// snapshotOf снимает содержимое материала для ревизии.
func snapshotOf(e *Entry) Snapshot {
	snap := Snapshot{Title: e.Title, Slug: e.Slug, SEOTitle: e.SEOTitle, SEODescription: e.SEODescription,
		Data: e.Data, CategoryIDs: []int64{}, TagIDs: []int64{}}
	for _, t := range e.Categories {
		snap.CategoryIDs = append(snap.CategoryIDs, t.ID)
	}
	for _, t := range e.Tags {
		snap.TagIDs = append(snap.TagIDs, t.ID)
	}
	return snap
}

func (s *Service) addRevision(ctx context.Context, q store.DBTX, actor access.Actor, e *Entry, note string) error {
	raw, err := json.Marshal(snapshotOf(e))
	if err != nil {
		return err
	}
	return store.NewRevisionRepo(q).Add(ctx, &store.Revision{
		EntryID: e.ID, Snapshot: string(raw), AuthorID: actor.ID, Note: note, CreatedAt: s.Now(),
	})
}

// Revisions — история материала, новые сверху.
func (s *Service) Revisions(ctx context.Context, id int64) ([]*Revision, error) {
	return store.NewRevisionRepo(s.DB).List(ctx, id)
}

// ParseSnapshot разбирает снимок ревизии.
func ParseSnapshot(rev *Revision) (Snapshot, error) {
	var snap Snapshot
	err := json.Unmarshal([]byte(rev.Snapshot), &snap)
	return snap, err
}

// Rollback восстанавливает содержимое ревизии number и записывает это новой ревизией.
func (s *Service) Rollback(ctx context.Context, actor access.Actor, id int64, number int) (*Entry, error) {
	var e *Entry
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		repo := store.NewEntryRepo(tx)
		var err error
		if e, err = repo.Get(ctx, id); err != nil {
			return err
		}
		if err := s.can(actor, "entry.rollback", resourceOf(e)); err != nil {
			return err
		}
		rev, err := store.NewRevisionRepo(tx).Get(ctx, id, number)
		if err != nil {
			return err
		}
		snap, err := ParseSnapshot(rev)
		if err != nil {
			return fmt.Errorf("revision %d: %w", number, err)
		}
		typ, err := s.typeOf(e.Type)
		if err != nil {
			return err
		}
		in := Input{Title: snap.Title, Slug: snap.Slug, SEOTitle: snap.SEOTitle, SEODescription: snap.SEODescription,
			Data: snap.Data, CategoryIDs: snap.CategoryIDs}
		// Старый снимок обязан пройти текущую схему: иначе откат нарушил бы инвариант 1.
		if err := s.prepare(typ, in, e); err != nil {
			return err
		}
		e.UpdatedAt = s.Now()
		if err := uniqueSlug(ctx, repo, e); err != nil {
			return err
		}
		if err := repo.Update(ctx, e); err != nil {
			return err
		}
		if err := s.restoreTerms(ctx, tx, e, snap); err != nil {
			return err
		}
		if err := s.addRevision(ctx, tx, actor, e, fmt.Sprintf("откат к ревизии %d", number)); err != nil {
			return err
		}
		return s.audit(ctx, tx, actor, "entry.rollback", e, fmt.Sprintf("к ревизии %d", number))
	})
	if err != nil {
		return nil, err
	}
	return e, nil
}
