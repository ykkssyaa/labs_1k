package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Session struct {
	TokenHash string
	UserID    int64
	CSRF      string
	ExpiresAt time.Time
}

type SessionRepo struct{ q DBTX }

func NewSessionRepo(q DBTX) *SessionRepo { return &SessionRepo{q: q} }

func (r *SessionRepo) Insert(ctx context.Context, s *Session) error {
	_, err := r.q.ExecContext(ctx, `INSERT INTO sessions (token_hash, user_id, csrf, expires_at) VALUES (?, ?, ?, ?)`,
		s.TokenHash, s.UserID, s.CSRF, s.ExpiresAt.UTC().Format(TimeFormat))
	return err
}

func (r *SessionRepo) Get(ctx context.Context, tokenHash string) (*Session, error) {
	var s Session
	var exp string
	err := r.q.QueryRowContext(ctx, `SELECT token_hash, user_id, csrf, expires_at FROM sessions WHERE token_hash = ?`,
		tokenHash).Scan(&s.TokenHash, &s.UserID, &s.CSRF, &exp)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	s.ExpiresAt = ParseTime(exp)
	return &s, err
}

func (r *SessionRepo) Delete(ctx context.Context, tokenHash string) error {
	_, err := r.q.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

func (r *SessionRepo) DeleteForUser(ctx context.Context, userID int64) error {
	_, err := r.q.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

func (r *SessionRepo) DeleteExpired(ctx context.Context) error {
	_, err := r.q.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, Now())
	return err
}
