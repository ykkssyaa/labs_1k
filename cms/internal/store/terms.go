package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type TermRepo struct{ q DBTX }

func NewTermRepo(q DBTX) *TermRepo { return &TermRepo{q: q} }

const termColumns = `id, vocabulary, name, slug, COALESCE(parent_id, 0)`

func scanTerm(sc interface{ Scan(...any) error }) (*Term, error) {
	var t Term
	if err := sc.Scan(&t.ID, &t.Vocabulary, &t.Name, &t.Slug, &t.ParentID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}

func nullID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

func (r *TermRepo) Insert(ctx context.Context, t *Term) error {
	res, err := r.q.ExecContext(ctx, `INSERT INTO terms (vocabulary, name, slug, parent_id) VALUES (?, ?, ?, ?)`,
		t.Vocabulary, t.Name, t.Slug, nullID(t.ParentID))
	if err != nil {
		return fmt.Errorf("insert term: %w", err)
	}
	t.ID, err = res.LastInsertId()
	return err
}

func (r *TermRepo) Update(ctx context.Context, t *Term) error {
	_, err := r.q.ExecContext(ctx, `UPDATE terms SET name=?, slug=?, parent_id=? WHERE id=?`,
		t.Name, t.Slug, nullID(t.ParentID), t.ID)
	return err
}

func (r *TermRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.q.ExecContext(ctx, `DELETE FROM terms WHERE id = ?`, id)
	return err
}

func (r *TermRepo) Get(ctx context.Context, id int64) (*Term, error) {
	return scanTerm(r.q.QueryRowContext(ctx, `SELECT `+termColumns+` FROM terms WHERE id = ?`, id))
}

func (r *TermRepo) BySlug(ctx context.Context, vocabulary, slug string) (*Term, error) {
	return scanTerm(r.q.QueryRowContext(ctx, `SELECT `+termColumns+` FROM terms WHERE vocabulary = ? AND slug = ?`,
		vocabulary, slug))
}

func (r *TermRepo) List(ctx context.Context, vocabulary string) ([]Term, error) {
	rows, err := r.q.QueryContext(ctx, `SELECT `+termColumns+` FROM terms WHERE vocabulary = ? ORDER BY name`, vocabulary)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Term
	for rows.Next() {
		t, err := scanTerm(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// ExistingIDs оставляет из ids только существующие термины словаря.
func (r *TermRepo) ExistingIDs(ctx context.Context, vocabulary string, ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	args := []any{vocabulary}
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := r.q.QueryContext(ctx, `SELECT id FROM terms WHERE vocabulary = ? AND id IN (?`+
		strings.Repeat(",?", len(ids)-1)+`) ORDER BY id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// SetEntryTerms заменяет термины словаря у материала.
func (r *TermRepo) SetEntryTerms(ctx context.Context, entryID int64, vocabulary string, termIDs []int64) error {
	if _, err := r.q.ExecContext(ctx, `DELETE FROM entry_terms WHERE entry_id = ?
		AND term_id IN (SELECT id FROM terms WHERE vocabulary = ?)`, entryID, vocabulary); err != nil {
		return err
	}
	for _, id := range termIDs {
		if _, err := r.q.ExecContext(ctx, `INSERT OR IGNORE INTO entry_terms (entry_id, term_id) VALUES (?, ?)`,
			entryID, id); err != nil {
			return fmt.Errorf("link term %d: %w", id, err)
		}
	}
	return nil
}

// EntryTerms — термины материала, отсортированные по словарю и имени.
func (r *TermRepo) EntryTerms(ctx context.Context, entryID int64) ([]Term, error) {
	rows, err := r.q.QueryContext(ctx, `SELECT t.id, t.vocabulary, t.name, t.slug, COALESCE(t.parent_id, 0)
		FROM terms t JOIN entry_terms et ON et.term_id = t.id WHERE et.entry_id = ? ORDER BY t.vocabulary, t.name`, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Term
	for rows.Next() {
		t, err := scanTerm(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// CountEntries — сколько опубликованных материалов у термина.
func (r *TermRepo) CountPublished(ctx context.Context, termID int64) (int, error) {
	var n int
	err := r.q.QueryRowContext(ctx, `SELECT COUNT(*) FROM entry_terms et JOIN entries e ON e.id = et.entry_id
		WHERE et.term_id = ? AND e.status = 'published'`, termID).Scan(&n)
	return n, err
}
