package store

import (
	"context"
	"database/sql"
	"errors"
)

type SettingsRepo struct{ q DBTX }

func NewSettingsRepo(q DBTX) *SettingsRepo { return &SettingsRepo{q: q} }

// Get возвращает значение настройки или def, если её нет.
func (r *SettingsRepo) Get(ctx context.Context, key, def string) (string, error) {
	var v string
	err := r.q.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return def, nil
	}
	return v, err
}

func (r *SettingsRepo) Set(ctx context.Context, key, value string) error {
	_, err := r.q.ExecContext(ctx, `INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`, key, value, Now())
	return err
}
