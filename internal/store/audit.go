package store

import (
	"context"
	"strings"
	"time"
)

type AuditRecord struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	UserName  string    `json:"user_name"`
	Action    string    `json:"action"`
	Object    string    `json:"object"`
	ObjectID  int64     `json:"object_id"`
	Details   string    `json:"details"`
	CreatedAt time.Time `json:"created_at"`
}

type AuditFilter struct {
	UserID int64
	Action string // точное действие или префикс с точкой: "entry." — все действия с материалами
	Limit  int
	Offset int
}

type AuditRepo struct{ q DBTX }

func NewAuditRepo(q DBTX) *AuditRepo { return &AuditRepo{q: q} }

func (r *AuditRepo) Add(ctx context.Context, rec *AuditRecord) error {
	_, err := r.q.ExecContext(ctx, `INSERT INTO audit_log (user_id, user_name, action, object, object_id, details, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, nullID(rec.UserID), rec.UserName, rec.Action, rec.Object, rec.ObjectID,
		rec.Details, Now())
	return err
}

func (r *AuditRepo) List(ctx context.Context, f AuditFilter) ([]AuditRecord, int, error) {
	var where []string
	var args []any
	if f.UserID != 0 {
		where, args = append(where, "user_id = ?"), append(args, f.UserID)
	}
	if f.Action != "" {
		if strings.HasSuffix(f.Action, ".") {
			where, args = append(where, "action LIKE ?"), append(args, escapeLike(f.Action)+"%")
			where[len(where)-1] += ` ESCAPE '\'`
		} else {
			where, args = append(where, "action = ?"), append(args, f.Action)
		}
	}
	cond := ""
	if len(where) > 0 {
		cond = " WHERE " + strings.Join(where, " AND ")
	}
	var total int
	if err := r.q.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_log`+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if f.Limit <= 0 {
		f.Limit = 50
	}
	rows, err := r.q.QueryContext(ctx, `SELECT id, COALESCE(user_id, 0), user_name, action, object, object_id, details, created_at
		FROM audit_log`+cond+` ORDER BY id DESC LIMIT ? OFFSET ?`, append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []AuditRecord
	for rows.Next() {
		var a AuditRecord
		var created string
		if err := rows.Scan(&a.ID, &a.UserID, &a.UserName, &a.Action, &a.Object, &a.ObjectID, &a.Details, &created); err != nil {
			return nil, 0, err
		}
		a.CreatedAt = ParseTime(created)
		out = append(out, a)
	}
	return out, total, rows.Err()
}
