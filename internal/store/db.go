// Package store — доступ к SQLite: подключение, миграции, репозитории.
package store

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// TimeFormat — формат хранения времени: строки RFC3339 в UTC сортируются как время.
const TimeFormat = time.RFC3339

// Open открывает (или создаёт) БД и применяет все миграции.
// path ":memory:" — БД в памяти для тестов.
func Open(path string) (*sql.DB, error) {
	dsn := path
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}
	dsn += "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	// SQLite — один писатель; одно соединение снимает гонки и делает :memory: общей.
	db.SetMaxOpenConns(1)
	if err := Migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// Migrate применяет ещё не применённые миграции из migrations/ по порядку имён.
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS db_migrations (
		name TEXT PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("create db_migrations: %w", err)
	}
	applied := map[string]bool{}
	rows, err := db.Query(`SELECT name FROM db_migrations`)
	if err != nil {
		return fmt.Errorf("read db_migrations: %w", err)
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		applied[name] = true
	}
	rows.Close()

	names, err := fs.Glob(migrationsFS, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, full := range names {
		name := strings.TrimPrefix(full, "migrations/")
		if applied[name] {
			continue
		}
		body, err := migrationsFS.ReadFile(full)
		if err != nil {
			return err
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if _, err := tx.Exec(`INSERT INTO db_migrations(name, applied_at) VALUES (?, ?)`,
			name, Now()); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}
	return nil
}

// AppliedMigrations возвращает имена применённых миграций по порядку.
func AppliedMigrations(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT name FROM db_migrations ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// Now — текущее время в формате хранения. Переменная, чтобы тесты могли зафиксировать время.
var Now = func() string { return time.Now().UTC().Format(TimeFormat) }

// ParseTime разбирает время из БД; пустая строка — нулевое время.
func ParseTime(s string) time.Time {
	t, _ := time.Parse(TimeFormat, s)
	return t
}
