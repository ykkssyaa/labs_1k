package content

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ykkssyaa/vestnik-cms/internal/access"
	"github.com/ykkssyaa/vestnik-cms/internal/store"
)

var ErrBadTransition = errors.New("переход недопустим из текущего статуса")

// Transition — именованный переход жизненного цикла.
type Transition struct {
	Name  string
	Label string
	From  string
	To    string
}

// Transitions — полная таблица переходов (см. spec/01-content-model.md, раздел 3).
var Transitions = []Transition{
	{"submit", "На модерацию", StatusDraft, StatusReview},
	{"publish", "Опубликовать", StatusReview, StatusPublished},
	{"reject", "Вернуть автору", StatusReview, StatusDraft},
	{"unpublish", "Снять с публикации", StatusPublished, StatusDraft},
	{"archive", "В архив", StatusPublished, StatusArchived},
	{"restore", "Из архива в черновики", StatusArchived, StatusDraft},
}

// StatusLabels — названия статусов для интерфейса.
var StatusLabels = map[string]string{
	StatusDraft:     "Черновик",
	StatusReview:    "На модерации",
	StatusPublished: "Опубликован",
	StatusArchived:  "В архиве",
}

func findTransition(name string) (Transition, bool) {
	for _, t := range Transitions {
		if t.Name == name {
			return t, true
		}
	}
	return Transition{}, false
}

// Available — переходы, которые actor может выполнить над материалом сейчас.
func (s *Service) Available(actor access.Actor, e *Entry) []Transition {
	var out []Transition
	for _, t := range Transitions {
		if t.From == e.Status && s.Auth.Can(actor, "entry.transition."+t.Name, resourceOf(e)) {
			out = append(out, t)
		}
	}
	return out
}

// Transition выполняет переход name над материалом id.
func (s *Service) Transition(ctx context.Context, actor access.Actor, id int64, name string) (*Entry, error) {
	tr, ok := findTransition(name)
	if !ok {
		return nil, fmt.Errorf("%w: неизвестный переход %q", ErrBadTransition, name)
	}
	var e *Entry
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		repo := store.NewEntryRepo(tx)
		var err error
		if e, err = repo.Get(ctx, id); err != nil {
			return err
		}
		if e.Status != tr.From {
			return fmt.Errorf("%w: %s из статуса %q", ErrBadTransition, name, e.Status)
		}
		// Права проверяются на материал в исходном статусе: «своё и в черновике» для автора.
		if err := s.can(actor, "entry.transition."+name, resourceOf(e)); err != nil {
			return err
		}
		e.Status = tr.To
		now := s.Now()
		if tr.To == StatusPublished && e.PublishedAt == nil {
			e.PublishedAt = &now
		}
		e.UpdatedAt = now
		if err := repo.UpdateStatus(ctx, e); err != nil {
			return err
		}
		return s.audit(ctx, tx, actor, "entry."+name, e, StatusLabels[tr.From]+" → "+StatusLabels[tr.To])
	})
	if err != nil {
		return nil, err
	}
	return e, s.loadTerms(ctx, e)
}
