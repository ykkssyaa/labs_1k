package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// DBTX — общее у *sql.DB и *sql.Tx: репозитории работают и внутри транзакции, и без неё.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

var ErrNotFound = errors.New("не найдено")

// Entry — материал. Поля типа лежат в Data, их смысл знает только схема типа.
type Entry struct {
	ID             int64          `json:"id"`
	Type           string         `json:"type"`
	Title          string         `json:"title"`
	Slug           string         `json:"slug"`
	Status         string         `json:"status"`
	AuthorID       int64          `json:"author_id"`
	AuthorName     string         `json:"author_name,omitempty"`
	SEOTitle       string         `json:"seo_title"`
	SEODescription string         `json:"seo_description"`
	Data           map[string]any `json:"data"`
	Categories     []Term         `json:"categories"`
	Tags           []Term         `json:"tags"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	PublishedAt    *time.Time     `json:"published_at,omitempty"`
	Search         string         `json:"-"`
}

// Revision — снимок содержимого материала.
type Revision struct {
	ID         int64     `json:"id"`
	EntryID    int64     `json:"entry_id"`
	Number     int       `json:"number"`
	Snapshot   string    `json:"snapshot"`
	AuthorID   int64     `json:"author_id"`
	AuthorName string    `json:"author_name,omitempty"`
	Note       string    `json:"note"`
	CreatedAt  time.Time `json:"created_at"`
}

// Term — рубрика или тег.
type Term struct {
	ID         int64  `json:"id"`
	Vocabulary string `json:"vocabulary"`
	Name       string `json:"name"`
	Slug       string `json:"slug"`
	ParentID   int64  `json:"parent_id,omitempty"`
}

func nullTime(s sql.NullString) *time.Time {
	if !s.Valid || s.String == "" {
		return nil
	}
	t := ParseTime(s.String)
	return &t
}

func timeOrNull(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(TimeFormat)
}
