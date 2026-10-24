package content

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/ykkssyaa/vestnik-cms/internal/access"
	"github.com/ykkssyaa/vestnik-cms/internal/store"
)

type Term = store.Term

const (
	VocabCategory = "category"
	VocabTag      = "tag"
)

var ErrTermCycle = errors.New("рубрика не может быть вложена сама в себя")

// Taxonomy — управление рубриками и тегами.
type Taxonomy struct {
	DB    *sql.DB
	Auth  access.Authorizer
	Audit AuditSink
}

func (t *Taxonomy) List(ctx context.Context, vocabulary string) ([]Term, error) {
	return store.NewTermRepo(t.DB).List(ctx, vocabulary)
}

func (t *Taxonomy) BySlug(ctx context.Context, vocabulary, slug string) (*Term, error) {
	return store.NewTermRepo(t.DB).BySlug(ctx, vocabulary, slug)
}

func checkTerm(term *Term) error {
	errs := FieldErrors{}
	term.Name = strings.TrimSpace(term.Name)
	if n := utf8.RuneCountInString(term.Name); n == 0 || n > 100 {
		errs["name"] = "название обязательно, до 100 символов"
	}
	if term.Slug == "" {
		term.Slug = Slugify(term.Name)
	}
	if !ValidSlug(term.Slug) {
		errs["slug"] = "только латиница в нижнем регистре, цифры и дефисы"
	}
	if term.Vocabulary != VocabCategory && term.Vocabulary != VocabTag {
		errs["vocabulary"] = "неизвестный словарь"
	}
	if term.Vocabulary == VocabTag && term.ParentID != 0 {
		errs["parent_id"] = "у тегов нет иерархии"
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// checkParent запрещает циклы: идём вверх от нового родителя и не должны встретить сам термин.
func checkParent(ctx context.Context, repo *store.TermRepo, term *Term) error {
	for id, depth := term.ParentID, 0; id != 0; depth++ {
		if id == term.ID || depth > 100 {
			return ErrTermCycle
		}
		parent, err := repo.Get(ctx, id)
		if err != nil {
			return fmt.Errorf("родитель %d: %w", id, err)
		}
		if parent.Vocabulary != term.Vocabulary {
			return fmt.Errorf("родитель из другого словаря")
		}
		id = parent.ParentID
	}
	return nil
}

func (t *Taxonomy) Create(ctx context.Context, actor access.Actor, term *Term) error {
	if !t.Auth.Can(actor, "taxonomy.manage", access.Resource{}) {
		return ErrForbidden
	}
	if err := checkTerm(term); err != nil {
		return err
	}
	tx, err := t.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	repo := store.NewTermRepo(tx)
	if err := checkParent(ctx, repo, term); err != nil {
		return err
	}
	if err := repo.Insert(ctx, term); err != nil {
		return err
	}
	if t.Audit != nil {
		if err := t.Audit.Record(ctx, tx, actor, "term.create", "term", term.ID, term.Name); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (t *Taxonomy) Update(ctx context.Context, actor access.Actor, term *Term) error {
	if !t.Auth.Can(actor, "taxonomy.manage", access.Resource{}) {
		return ErrForbidden
	}
	if err := checkTerm(term); err != nil {
		return err
	}
	tx, err := t.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	repo := store.NewTermRepo(tx)
	if err := checkParent(ctx, repo, term); err != nil {
		return err
	}
	if err := repo.Update(ctx, term); err != nil {
		return err
	}
	if t.Audit != nil {
		if err := t.Audit.Record(ctx, tx, actor, "term.update", "term", term.ID, term.Name); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Delete удаляет термин; связи с материалами снимает каскад в БД.
func (t *Taxonomy) Delete(ctx context.Context, actor access.Actor, id int64) error {
	if !t.Auth.Can(actor, "taxonomy.manage", access.Resource{}) {
		return ErrForbidden
	}
	tx, err := t.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := store.NewTermRepo(tx).Delete(ctx, id); err != nil {
		return err
	}
	if t.Audit != nil {
		if err := t.Audit.Record(ctx, tx, actor, "term.delete", "term", id, ""); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ensureTags находит или создаёт теги по именам. Создание тегов при сохранении
// материала не требует права taxonomy.manage: автор ставит теги, но не правит рубрики.
func ensureTags(ctx context.Context, repo *store.TermRepo, names []string) ([]int64, error) {
	var ids []int64
	seen := map[string]bool{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		slug := Slugify(name)
		if name == "" || slug == "" || seen[slug] {
			continue
		}
		seen[slug] = true
		term, err := repo.BySlug(ctx, VocabTag, slug)
		if errors.Is(err, store.ErrNotFound) {
			term = &Term{Vocabulary: VocabTag, Name: name, Slug: slug}
			err = repo.Insert(ctx, term)
		}
		if err != nil {
			return nil, err
		}
		ids = append(ids, term.ID)
	}
	return ids, nil
}

// saveTerms записывает рубрики и теги материала из ввода.
func (s *Service) saveTerms(ctx context.Context, q store.DBTX, e *Entry, in Input) error {
	repo := store.NewTermRepo(q)
	cats, err := repo.ExistingIDs(ctx, VocabCategory, in.CategoryIDs)
	if err != nil {
		return err
	}
	tags, err := ensureTags(ctx, repo, in.Tags)
	if err != nil {
		return err
	}
	return s.setTerms(ctx, repo, e, cats, tags)
}

// restoreTerms возвращает термины из снимка; удалённые с тех пор термины пропускаются.
func (s *Service) restoreTerms(ctx context.Context, q store.DBTX, e *Entry, snap Snapshot) error {
	repo := store.NewTermRepo(q)
	cats, err := repo.ExistingIDs(ctx, VocabCategory, snap.CategoryIDs)
	if err != nil {
		return err
	}
	tags, err := repo.ExistingIDs(ctx, VocabTag, snap.TagIDs)
	if err != nil {
		return err
	}
	return s.setTerms(ctx, repo, e, cats, tags)
}

func (s *Service) setTerms(ctx context.Context, repo *store.TermRepo, e *Entry, cats, tags []int64) error {
	if err := repo.SetEntryTerms(ctx, e.ID, VocabCategory, cats); err != nil {
		return err
	}
	if err := repo.SetEntryTerms(ctx, e.ID, VocabTag, tags); err != nil {
		return err
	}
	return s.loadTermsQ(ctx, repo, e)
}

func (s *Service) loadTerms(ctx context.Context, e *Entry) error {
	return s.loadTermsQ(ctx, store.NewTermRepo(s.DB), e)
}

func (s *Service) loadTermsQ(ctx context.Context, repo *store.TermRepo, e *Entry) error {
	terms, err := repo.EntryTerms(ctx, e.ID)
	if err != nil {
		return err
	}
	e.Categories, e.Tags = []Term{}, []Term{}
	for _, t := range terms {
		if t.Vocabulary == VocabCategory {
			e.Categories = append(e.Categories, t)
		} else {
			e.Tags = append(e.Tags, t)
		}
	}
	return nil
}
