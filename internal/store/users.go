package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	Role         string    `json:"role"`
	PasswordHash string    `json:"-"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
}

type UserRepo struct{ q DBTX }

func NewUserRepo(q DBTX) *UserRepo { return &UserRepo{q: q} }

const userColumns = `id, email, name, role, password_hash, active, created_at`

func scanUser(sc interface{ Scan(...any) error }) (*User, error) {
	var u User
	var created string
	if err := sc.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.PasswordHash, &u.Active, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	u.CreatedAt = ParseTime(created)
	return &u, nil
}

func (r *UserRepo) Insert(ctx context.Context, u *User) error {
	res, err := r.q.ExecContext(ctx, `INSERT INTO users (email, name, role, password_hash, active, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, u.Email, u.Name, u.Role, u.PasswordHash, u.Active, Now())
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	u.ID, err = res.LastInsertId()
	return err
}

func (r *UserRepo) Update(ctx context.Context, u *User) error {
	_, err := r.q.ExecContext(ctx, `UPDATE users SET email=?, name=?, role=?, password_hash=?, active=? WHERE id=?`,
		u.Email, u.Name, u.Role, u.PasswordHash, u.Active, u.ID)
	return err
}

func (r *UserRepo) Get(ctx context.Context, id int64) (*User, error) {
	return scanUser(r.q.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id = ?`, id))
}

func (r *UserRepo) ByEmail(ctx context.Context, email string) (*User, error) {
	return scanUser(r.q.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE email = ?`, email))
}

func (r *UserRepo) List(ctx context.Context) ([]*User, error) {
	rows, err := r.q.QueryContext(ctx, `SELECT `+userColumns+` FROM users ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (r *UserRepo) Count(ctx context.Context) (int, error) {
	var n int
	err := r.q.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}
