package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Media struct {
	ID         int64              `json:"id"`
	Filename   string             `json:"filename"`
	Path       string             `json:"path"`
	MIME       string             `json:"mime"`
	Size       int64              `json:"size"`
	Width      int                `json:"width"`
	Height     int                `json:"height"`
	Alt        string             `json:"alt"`
	UploadedBy int64              `json:"uploaded_by"`
	CreatedAt  time.Time          `json:"created_at"`
	Variants   map[string]Variant `json:"variants"`
}

type Variant struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type MediaRepo struct{ q DBTX }

func NewMediaRepo(q DBTX) *MediaRepo { return &MediaRepo{q: q} }

func (r *MediaRepo) Insert(ctx context.Context, m *Media) error {
	res, err := r.q.ExecContext(ctx, `INSERT INTO media (filename, path, mime, size, width, height, alt, uploaded_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, m.Filename, m.Path, m.MIME, m.Size, m.Width, m.Height, m.Alt,
		nullID(m.UploadedBy), m.CreatedAt.UTC().Format(TimeFormat))
	if err != nil {
		return err
	}
	if m.ID, err = res.LastInsertId(); err != nil {
		return err
	}
	for _, v := range m.Variants {
		if _, err := r.q.ExecContext(ctx, `INSERT INTO media_variants (media_id, name, path, width, height)
			VALUES (?, ?, ?, ?, ?)`, m.ID, v.Name, v.Path, v.Width, v.Height); err != nil {
			return err
		}
	}
	return nil
}

func (r *MediaRepo) UpdateAlt(ctx context.Context, id int64, alt string) error {
	_, err := r.q.ExecContext(ctx, `UPDATE media SET alt = ? WHERE id = ?`, alt, id)
	return err
}

func (r *MediaRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.q.ExecContext(ctx, `DELETE FROM media WHERE id = ?`, id)
	return err
}

const mediaColumns = `id, filename, path, mime, size, width, height, alt, COALESCE(uploaded_by, 0), created_at`

func (r *MediaRepo) scan(sc interface{ Scan(...any) error }) (*Media, error) {
	var m Media
	var created string
	if err := sc.Scan(&m.ID, &m.Filename, &m.Path, &m.MIME, &m.Size, &m.Width, &m.Height, &m.Alt,
		&m.UploadedBy, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	m.CreatedAt = ParseTime(created)
	return &m, nil
}

func (r *MediaRepo) loadVariants(ctx context.Context, m *Media) error {
	rows, err := r.q.QueryContext(ctx, `SELECT name, path, width, height FROM media_variants WHERE media_id = ?`, m.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	m.Variants = map[string]Variant{}
	for rows.Next() {
		var v Variant
		if err := rows.Scan(&v.Name, &v.Path, &v.Width, &v.Height); err != nil {
			return err
		}
		m.Variants[v.Name] = v
	}
	return rows.Err()
}

func (r *MediaRepo) Get(ctx context.Context, id int64) (*Media, error) {
	m, err := r.scan(r.q.QueryRowContext(ctx, `SELECT `+mediaColumns+` FROM media WHERE id = ?`, id))
	if err != nil {
		return nil, err
	}
	return m, r.loadVariants(ctx, m)
}

func (r *MediaRepo) List(ctx context.Context, limit, offset int) ([]*Media, int, error) {
	var total int
	if err := r.q.QueryRowContext(ctx, `SELECT COUNT(*) FROM media`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.q.QueryContext(ctx, `SELECT `+mediaColumns+` FROM media ORDER BY id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	var out []*Media
	for rows.Next() {
		m, err := r.scan(rows)
		if err != nil {
			rows.Close()
			return nil, 0, err
		}
		out = append(out, m)
	}
	rows.Close()
	for _, m := range out {
		if err := r.loadVariants(ctx, m); err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}
