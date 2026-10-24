package store

import (
	"path/filepath"
	"testing"
)

func TestMigrateIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	first, err := AppliedMigrations(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) == 0 {
		t.Fatal("ни одной миграции не применено")
	}
	db.Close()

	db, err = Open(path) // повторный старт не должен падать и применять заново
	if err != nil {
		t.Fatalf("повторное открытие: %v", err)
	}
	defer db.Close()
	second, _ := AppliedMigrations(db)
	if len(second) != len(first) {
		t.Fatalf("миграции применились повторно: %v -> %v", first, second)
	}
}

func TestForeignKeysEnabled(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var on int
	if err := db.QueryRow(`PRAGMA foreign_keys`).Scan(&on); err != nil {
		t.Fatal(err)
	}
	if on != 1 {
		t.Fatal("внешние ключи выключены")
	}
}
