package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type EntryRepo struct{ q DBTX }

func NewEntryRepo(q DBTX) *EntryRepo { return &EntryRepo{q: q} }

// EntryFilter — условия выборки списка материалов. Пустые поля не фильтруют.
type EntryFilter struct {
	Type       string
	Types      []string
	Status     string
	AuthorID   int64
	CategoryID int64
	TagID      int64
	Query      string // подстрока, ищется по полю search (нижний регистр)
	Sort       string // "updated" (по умолчанию), "published", "title"
	Limit      int
	Offset     int
}

const entryColumns = `e.id, e.type, e.title, e.slug, e.status, e.author_id, COALESCE(u.name, ''),
	e.seo_title, e.seo_description, e.data, e.created_at, e.updated_at, e.published_at`

const entryFrom = ` FROM entries e LEFT JOIN users u ON u.id = e.author_id`

func scanEntry(sc interface{ Scan(...any) error }) (*Entry, error) {
	var e Entry
	var data, created, updated string
	var published sql.NullString
	if err := sc.Scan(&e.ID, &e.Type, &e.Title, &e.Slug, &e.Status, &e.AuthorID, &e.AuthorName,
		&e.SEOTitle, &e.SEODescription, &data, &created, &updated, &published); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(data), &e.Data); err != nil {
		return nil, fmt.Errorf("entry %d: data: %w", e.ID, err)
	}
	e.CreatedAt, e.UpdatedAt = ParseTime(created), ParseTime(updated)
	e.PublishedAt = nullTime(published)
	return &e, nil
}

func (r *EntryRepo) Insert(ctx context.Context, e *Entry) error {
	data, err := json.Marshal(e.Data)
	if err != nil {
		return err
	}
	res, err := r.q.ExecContext(ctx, `INSERT INTO entries
		(type, title, slug, status, author_id, seo_title, seo_description, data, search, created_at, updated_at, published_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Type, e.Title, e.Slug, e.Status, e.AuthorID, e.SEOTitle, e.SEODescription, string(data), e.Search,
		e.CreatedAt.UTC().Format(TimeFormat), e.UpdatedAt.UTC().Format(TimeFormat), timeOrNull(e.PublishedAt))
	if err != nil {
		return fmt.Errorf("insert entry: %w", err)
	}
	e.ID, err = res.LastInsertId()
	return err
}

func (r *EntryRepo) Update(ctx context.Context, e *Entry) error {
	data, err := json.Marshal(e.Data)
	if err != nil {
		return err
	}
	res, err := r.q.ExecContext(ctx, `UPDATE entries SET title=?, slug=?, status=?, seo_title=?, seo_description=?,
		data=?, search=?, updated_at=?, published_at=? WHERE id=?`,
		e.Title, e.Slug, e.Status, e.SEOTitle, e.SEODescription, string(data), e.Search,
		e.UpdatedAt.UTC().Format(TimeFormat), timeOrNull(e.PublishedAt), e.ID)
	if err != nil {
		return fmt.Errorf("update entry: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateStatus меняет только статус и даты: переход жизненного цикла не трогает содержимое.
func (r *EntryRepo) UpdateStatus(ctx context.Context, e *Entry) error {
	res, err := r.q.ExecContext(ctx, `UPDATE entries SET status=?, updated_at=?, published_at=? WHERE id=?`,
		e.Status, e.UpdatedAt.UTC().Format(TimeFormat), timeOrNull(e.PublishedAt), e.ID)
	if err != nil {
		return fmt.Errorf("update entry status: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *EntryRepo) Get(ctx context.Context, id int64) (*Entry, error) {
	e, err := scanEntry(r.q.QueryRowContext(ctx, `SELECT `+entryColumns+entryFrom+` WHERE e.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return e, err
}

func (r *EntryRepo) GetBySlug(ctx context.Context, typ, slug string) (*Entry, error) {
	e, err := scanEntry(r.q.QueryRowContext(ctx,
		`SELECT `+entryColumns+entryFrom+` WHERE e.type = ? AND e.slug = ?`, typ, slug))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return e, err
}

// SlugTaken — занят ли slug в пределах типа другим материалом.
func (r *EntryRepo) SlugTaken(ctx context.Context, typ, slug string, exceptID int64) (bool, error) {
	var n int
	err := r.q.QueryRowContext(ctx, `SELECT COUNT(*) FROM entries WHERE type=? AND slug=? AND id<>?`,
		typ, slug, exceptID).Scan(&n)
	return n > 0, err
}

func (r *EntryRepo) Delete(ctx context.Context, id int64) error {
	res, err := r.q.ExecContext(ctx, `DELETE FROM entries WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// List возвращает страницу материалов и общее число подходящих под фильтр.
func (r *EntryRepo) List(ctx context.Context, f EntryFilter) ([]*Entry, int, error) {
	var where []string
	var args []any
	if f.Type != "" {
		where, args = append(where, "e.type = ?"), append(args, f.Type)
	}
	if len(f.Types) > 0 {
		where = append(where, "e.type IN (?"+strings.Repeat(",?", len(f.Types)-1)+")")
		for _, t := range f.Types {
			args = append(args, t)
		}
	}
	if f.Status != "" {
		where, args = append(where, "e.status = ?"), append(args, f.Status)
	}
	if f.AuthorID != 0 {
		where, args = append(where, "e.author_id = ?"), append(args, f.AuthorID)
	}
	if f.CategoryID != 0 {
		where = append(where, "EXISTS (SELECT 1 FROM entry_terms et WHERE et.entry_id = e.id AND et.term_id = ?)")
		args = append(args, f.CategoryID)
	}
	if f.TagID != 0 {
		where = append(where, "EXISTS (SELECT 1 FROM entry_terms et WHERE et.entry_id = e.id AND et.term_id = ?)")
		args = append(args, f.TagID)
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		where = append(where, "e.search LIKE ? ESCAPE '\\'")
		args = append(args, "%"+escapeLike(strings.ToLower(q))+"%")
	}
	cond := ""
	if len(where) > 0 {
		cond = " WHERE " + strings.Join(where, " AND ")
	}
	var total int
	if err := r.q.QueryRowContext(ctx, `SELECT COUNT(*) FROM entries e`+cond, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count entries: %w", err)
	}
	order := " ORDER BY e.updated_at DESC, e.id DESC"
	switch f.Sort {
	case "published":
		order = " ORDER BY e.published_at DESC, e.id DESC"
	case "title":
		order = " ORDER BY e.title, e.id"
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.q.QueryContext(ctx, `SELECT `+entryColumns+entryFrom+cond+order+` LIMIT ? OFFSET ?`,
		append(args, limit, f.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list entries: %w", err)
	}
	defer rows.Close()
	var out []*Entry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

type RevisionRepo struct{ q DBTX }

func NewRevisionRepo(q DBTX) *RevisionRepo { return &RevisionRepo{q: q} }

// Add записывает ревизию со следующим номером. Вызывать внутри транзакции
// сохранения материала, тогда номера идут подряд без гонок.
func (r *RevisionRepo) Add(ctx context.Context, rev *Revision) error {
	if err := r.q.QueryRowContext(ctx, `SELECT COALESCE(MAX(number), 0) + 1 FROM entry_revisions WHERE entry_id = ?`,
		rev.EntryID).Scan(&rev.Number); err != nil {
		return err
	}
	res, err := r.q.ExecContext(ctx, `INSERT INTO entry_revisions (entry_id, number, snapshot, author_id, note, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, rev.EntryID, rev.Number, rev.Snapshot, rev.AuthorID, rev.Note,
		rev.CreatedAt.UTC().Format(TimeFormat))
	if err != nil {
		return fmt.Errorf("insert revision: %w", err)
	}
	rev.ID, err = res.LastInsertId()
	return err
}

func (r *RevisionRepo) List(ctx context.Context, entryID int64) ([]*Revision, error) {
	rows, err := r.q.QueryContext(ctx, `SELECT r.id, r.entry_id, r.number, r.snapshot, r.author_id,
		COALESCE(u.name, ''), r.note, r.created_at FROM entry_revisions r LEFT JOIN users u ON u.id = r.author_id
		WHERE r.entry_id = ? ORDER BY r.number DESC`, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Revision
	for rows.Next() {
		var rev Revision
		var created string
		if err := rows.Scan(&rev.ID, &rev.EntryID, &rev.Number, &rev.Snapshot, &rev.AuthorID, &rev.AuthorName,
			&rev.Note, &created); err != nil {
			return nil, err
		}
		rev.CreatedAt = ParseTime(created)
		out = append(out, &rev)
	}
	return out, rows.Err()
}

func (r *RevisionRepo) Get(ctx context.Context, entryID int64, number int) (*Revision, error) {
	var rev Revision
	var created string
	err := r.q.QueryRowContext(ctx, `SELECT id, entry_id, number, snapshot, author_id, note, created_at
		FROM entry_revisions WHERE entry_id = ? AND number = ?`, entryID, number).
		Scan(&rev.ID, &rev.EntryID, &rev.Number, &rev.Snapshot, &rev.AuthorID, &rev.Note, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	rev.CreatedAt = ParseTime(created)
	return &rev, err
}
