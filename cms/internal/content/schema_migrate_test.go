package content

import (
	"errors"
	"testing"
)

const noteV2 = `{
  "name": "note", "version": 2, "label": "Заметка", "url_prefix": "notes",
  "fields": [
    {"name": "text", "label": "Текст", "type": "markdown", "required": true},
    {"name": "mood", "label": "Настроение", "type": "select", "options": ["хорошее", "плохое"], "default": "хорошее"}
  ],
  "migrations": [
    {"to_version": 2, "ops": [
      {"op": "rename_field", "field": "body", "to": "text"},
      {"op": "add_field", "field": "mood"}
    ]}
  ]
}`

func TestSchemaMigrationRenamesAndAddsFields(t *testing.T) {
	f := newFixture(t)
	if _, err := f.svc.Sync(f.ctx); err != nil { // зафиксировать версии v1
		t.Fatal(err)
	}
	e, err := f.svc.Create(f.ctx, f.author, "note", Input{Title: "Старая заметка", Data: map[string]any{"body": "текст v1"}})
	if err != nil {
		t.Fatal(err)
	}

	// «Выкатили» новую версию схемы.
	reg, _ := NewRegistry(mustType(t, articleSchema), mustType(t, noteV2))
	f.svc.Types = reg
	results, err := f.svc.Sync(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].From != 1 || results[0].To != 2 || results[0].Affected != 1 {
		t.Fatalf("неожиданный результат синхронизации: %+v", results)
	}
	got, _ := f.svc.Get(f.ctx, e.ID)
	if got.Data["text"] != "текст v1" || got.Data["mood"] != "хорошее" {
		t.Fatalf("данные не мигрированы: %#v", got.Data)
	}
	if _, ok := got.Data["body"]; ok {
		t.Fatal("старое поле осталось")
	}
	revs, _ := f.svc.Revisions(f.ctx, e.ID)
	if revs[0].Note != "миграция схемы v2" {
		t.Fatalf("миграция не записана ревизией: %q", revs[0].Note)
	}
	var steps int
	f.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE type_name = 'note'`).Scan(&steps)
	if steps != 2 {
		t.Fatalf("в schema_migrations %d шагов, ожидали 2", steps)
	}
	// Повторный Sync ничего не делает.
	if again, _ := f.svc.Sync(f.ctx); len(again) != 0 {
		t.Fatalf("повторная синхронизация что-то сделала: %+v", again)
	}
}

func TestSchemaDowngradeRejected(t *testing.T) {
	f := newFixture(t)
	reg, _ := NewRegistry(mustType(t, articleSchema), mustType(t, noteV2))
	f.svc.Types = reg
	if _, err := f.svc.Sync(f.ctx); err != nil {
		t.Fatal(err)
	}
	reg1, _ := NewRegistry(mustType(t, articleSchema), mustType(t, noteSchema))
	f.svc.Types = reg1
	if _, err := f.svc.Sync(f.ctx); !errors.Is(err, ErrSchemaDowngrade) {
		t.Fatalf("понижение версии должно быть ошибкой, получили %v", err)
	}
}
