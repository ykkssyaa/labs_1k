package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/ykkssyaa/vestnik-cms/internal/access"
	"github.com/ykkssyaa/vestnik-cms/internal/store"
)

const articleSchema = `{
  "name": "article", "version": 1, "label": "Статья", "url_prefix": "articles",
  "fields": [
    {"name": "lead", "label": "Лид", "type": "text", "required": true, "max_length": 300},
    {"name": "body", "label": "Текст", "type": "markdown", "required": true},
    {"name": "reading_time", "label": "Время", "type": "int", "min": 1, "max": 120},
    {"name": "is_hot", "label": "Горячее", "type": "bool"}
  ]
}`

const noteSchema = `{
  "name": "note", "version": 1, "label": "Заметка", "url_prefix": "notes",
  "fields": [{"name": "body", "label": "Текст", "type": "markdown", "required": true}]
}`

type fixture struct {
	db     *sql.DB
	svc    *Service
	tax    *Taxonomy
	author access.Actor
	ctx    context.Context
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	reg, err := NewRegistry(mustType(t, articleSchema), mustType(t, noteSchema))
	if err != nil {
		t.Fatal(err)
	}
	u := &store.User{Email: "a@x", Name: "Автор", Role: "author", PasswordHash: "-", Active: true}
	if err := store.NewUserRepo(db).Insert(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	svc := NewService(db, reg, access.AllowAll{})
	clock := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	svc.Now = func() time.Time { clock = clock.Add(time.Minute); return clock }
	return &fixture{db: db, svc: svc, tax: &Taxonomy{DB: db, Auth: access.AllowAll{}},
		author: access.Actor{ID: u.ID, Name: u.Name, Role: u.Role}, ctx: context.Background()}
}

func articleInput(title string) Input {
	return Input{Title: title, Data: map[string]any{"lead": "Коротко", "body": "Текст **статьи**", "reading_time": "5"}}
}

func (f *fixture) create(t *testing.T, in Input) *Entry {
	t.Helper()
	e, err := f.svc.Create(f.ctx, f.author, "article", in)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return e
}

func (f *fixture) category(t *testing.T, name string, parent int64) *Term {
	t.Helper()
	term := &Term{Vocabulary: VocabCategory, Name: name, ParentID: parent}
	if err := f.tax.Create(f.ctx, f.author, term); err != nil {
		t.Fatalf("category: %v", err)
	}
	return term
}

// Инвариант 1: материал не сохраняется, если данные не проходят схему.
func TestCreateRejectsInvalidData(t *testing.T) {
	f := newFixture(t)
	in := articleInput("Заголовок")
	in.Data["reading_time"] = "500"
	delete(in.Data, "body")
	_, err := f.svc.Create(f.ctx, f.author, "article", in)
	var fe FieldErrors
	if !errors.As(err, &fe) || fe["data.reading_time"] == "" || fe["data.body"] == "" {
		t.Fatalf("ожидались ошибки по reading_time и body, получили %v", err)
	}
	if _, total, _ := f.svc.List(f.ctx, store.EntryFilter{}); total != 0 {
		t.Fatalf("невалидный материал сохранён")
	}
	if _, err := f.svc.Create(f.ctx, f.author, "nope", in); !errors.Is(err, ErrUnknownType) {
		t.Fatalf("неизвестный тип: %v", err)
	}
}

// Инвариант 2: slug уникален в пределах типа, но может совпадать у разных типов.
func TestSlugGeneratedAndUniquePerType(t *testing.T) {
	f := newFixture(t)
	a := f.create(t, articleInput("Новый мост через Неву"))
	if a.Slug != "novyy-most-cherez-nevu" {
		t.Fatalf("slug из заголовка: %q", a.Slug)
	}
	b := f.create(t, articleInput("Новый мост через Неву"))
	if b.Slug != "novyy-most-cherez-nevu-2" {
		t.Fatalf("конфликт slug не разрешён: %q", b.Slug)
	}
	n, err := f.svc.Create(f.ctx, f.author, "note", Input{Title: "Новый мост через Неву", Data: map[string]any{"body": "x"}})
	if err != nil || n.Slug != a.Slug {
		t.Fatalf("в другом типе slug может совпадать: %v %q", err, n.Slug)
	}
	bad := articleInput("X")
	bad.Slug = "Плохой slug"
	if _, err := f.svc.Create(f.ctx, f.author, "article", bad); err == nil {
		t.Fatal("кириллический slug должен отклоняться")
	}
}

// Инвариант 3: неизвестные поля не хранятся.
func TestUnknownFieldsAreNotStored(t *testing.T) {
	f := newFixture(t)
	in := articleInput("Заголовок")
	in.Data["admin"] = true
	e := f.create(t, in)
	got, _ := f.svc.Get(f.ctx, e.ID)
	if _, ok := got.Data["admin"]; ok {
		t.Fatal("неизвестное поле сохранилось")
	}
}

// Инвариант 4: статус меняется только по таблице переходов. Проверяем все пары.
func TestTransitionMatrix(t *testing.T) {
	statuses := []string{StatusDraft, StatusReview, StatusPublished, StatusArchived}
	path := map[string][]string{ // как довести материал до статуса
		StatusDraft:     nil,
		StatusReview:    {"submit"},
		StatusPublished: {"submit", "publish"},
		StatusArchived:  {"submit", "publish", "archive"},
	}
	for _, from := range statuses {
		for _, tr := range Transitions {
			t.Run(from+"/"+tr.Name, func(t *testing.T) {
				f := newFixture(t)
				e := f.create(t, articleInput("Т"))
				for _, step := range path[from] {
					if _, err := f.svc.Transition(f.ctx, f.author, e.ID, step); err != nil {
						t.Fatalf("подготовка %s: %v", step, err)
					}
				}
				got, err := f.svc.Transition(f.ctx, f.author, e.ID, tr.Name)
				if tr.From == from {
					if err != nil || got.Status != tr.To {
						t.Fatalf("разрешённый переход не выполнен: %v", err)
					}
				} else if !errors.Is(err, ErrBadTransition) {
					t.Fatalf("недопустимый переход выполнен: err=%v", err)
				}
			})
		}
	}
}

// Инвариант 5: published_at ставится при первой публикации и не меняется.
func TestPublishedAtSetOnce(t *testing.T) {
	f := newFixture(t)
	e := f.create(t, articleInput("Т"))
	for _, step := range []string{"submit", "publish"} {
		e, _ = f.svc.Transition(f.ctx, f.author, e.ID, step)
	}
	first := *e.PublishedAt
	for _, step := range []string{"unpublish", "submit", "publish"} {
		var err error
		if e, err = f.svc.Transition(f.ctx, f.author, e.ID, step); err != nil {
			t.Fatal(err)
		}
	}
	if !e.PublishedAt.Equal(first) {
		t.Fatalf("published_at изменился: %v -> %v", first, e.PublishedAt)
	}
}

// Инвариант 6: номера ревизий подряд; смена статуса ревизию не создаёт.
func TestRevisionNumbersSequential(t *testing.T) {
	f := newFixture(t)
	e := f.create(t, articleInput("v1"))
	f.svc.Update(f.ctx, f.author, e.ID, articleInput("v2"))
	f.svc.Transition(f.ctx, f.author, e.ID, "submit")
	f.svc.Update(f.ctx, f.author, e.ID, articleInput("v3"))
	f.svc.Rollback(f.ctx, f.author, e.ID, 1)
	revs, err := f.svc.Revisions(f.ctx, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(revs) != 4 {
		t.Fatalf("ожидали 4 ревизии (создание, 2 правки, откат), получили %d", len(revs))
	}
	for i, r := range revs { // список отсортирован от новых к старым
		if r.Number != len(revs)-i {
			t.Fatalf("номера не подряд: позиция %d номер %d", i, r.Number)
		}
	}
}

// Инвариант 7: Entry -> JSON -> Entry даёт тот же материал (после приведения к схеме).
func TestEntryJSONRoundTrip(t *testing.T) {
	f := newFixture(t)
	cat := f.category(t, "Город", 0)
	in := articleInput("Круговая поездка")
	in.CategoryIDs = []int64{cat.ID}
	in.Tags = []string{"Транспорт", "Метро"}
	e := f.create(t, in)
	orig, _ := f.svc.Get(f.ctx, e.ID)

	raw, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}
	var back Entry
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	typ, _ := f.svc.Types.Get(back.Type)
	back.Data = typ.Normalize(back.Data)
	if !reflect.DeepEqual(*orig, back) {
		t.Fatalf("round-trip изменил материал:\n было %#v\nстало %#v", *orig, back)
	}
}

// Снимок ревизии -> материал -> снимок даёт тот же снимок.
func TestSnapshotRoundTrip(t *testing.T) {
	f := newFixture(t)
	in := articleInput("Снимок")
	in.Tags = []string{"один"}
	e := f.create(t, in)
	revs, _ := f.svc.Revisions(f.ctx, e.ID)
	snap, err := ParseSnapshot(revs[0])
	if err != nil {
		t.Fatal(err)
	}
	got, _ := f.svc.Get(f.ctx, e.ID)
	typ, _ := f.svc.Types.Get("article")
	snap.Data = typ.Normalize(snap.Data)
	if want := snapshotOf(got); !reflect.DeepEqual(snap, want) {
		t.Fatalf("снимок не совпадает с материалом:\n%#v\n%#v", snap, want)
	}
}

// Инвариант 8: откат восстанавливает ровно снимок, включая термины.
func TestRollbackRestoresSnapshotWithTerms(t *testing.T) {
	f := newFixture(t)
	c1, c2 := f.category(t, "Город", 0), f.category(t, "Культура", 0)
	in := articleInput("Версия 1")
	in.CategoryIDs, in.Tags = []int64{c1.ID}, []string{"старый"}
	e := f.create(t, in)
	v1, _ := f.svc.Get(f.ctx, e.ID)

	in2 := articleInput("Версия 2")
	in2.Data["lead"] = "Другой лид"
	in2.CategoryIDs, in2.Tags = []int64{c2.ID}, []string{"новый"}
	if _, err := f.svc.Update(f.ctx, f.author, e.ID, in2); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Rollback(f.ctx, f.author, e.ID, 1); err != nil {
		t.Fatal(err)
	}
	got, _ := f.svc.Get(f.ctx, e.ID)
	if !reflect.DeepEqual(snapshotOf(got), snapshotOf(v1)) {
		t.Fatalf("откат не восстановил версию 1:\n%#v\n%#v", snapshotOf(got), snapshotOf(v1))
	}
	if _, err := f.svc.Rollback(f.ctx, f.author, e.ID, 99); !errors.Is(err, ErrNotFound) {
		t.Fatalf("откат к несуществующей ревизии: %v", err)
	}
}

// Поиск должен находить материал и после смены статуса, без учёта регистра кириллицы.
func TestSearchSurvivesTransitionAndIgnoresCase(t *testing.T) {
	f := newFixture(t)
	e := f.create(t, articleInput("Эрмитаж открыл новый зал"))
	f.svc.Transition(f.ctx, f.author, e.ID, "submit")
	for _, q := range []string{"эрмитаж", "ЭРМИТАЖ", "статьи"} {
		if _, total, _ := f.svc.List(f.ctx, store.EntryFilter{Query: q}); total != 1 {
			t.Fatalf("поиск %q после submit нашёл %d", q, total)
		}
	}
	if _, total, _ := f.svc.List(f.ctx, store.EntryFilter{Query: "100%"}); total != 0 {
		t.Fatal("символ % в запросе должен экранироваться")
	}
}

func TestListFilters(t *testing.T) {
	f := newFixture(t)
	city := f.category(t, "Город", 0)
	a := articleInput("А")
	a.CategoryIDs, a.Tags = []int64{city.ID}, []string{"мосты"}
	ea := f.create(t, a)
	f.create(t, articleInput("Б"))
	f.svc.Create(f.ctx, f.author, "note", Input{Title: "В", Data: map[string]any{"body": "x"}})
	f.svc.Transition(f.ctx, f.author, ea.ID, "submit")
	tag, _ := f.tax.BySlug(f.ctx, VocabTag, "mosty")

	cases := []struct {
		name   string
		filter store.EntryFilter
		want   int
	}{
		{"all", store.EntryFilter{}, 3},
		{"by type", store.EntryFilter{Type: "article"}, 2},
		{"by types", store.EntryFilter{Types: []string{"article", "note"}}, 3},
		{"by status", store.EntryFilter{Status: StatusReview}, 1},
		{"by author", store.EntryFilter{AuthorID: f.author.ID}, 3},
		{"by category", store.EntryFilter{CategoryID: city.ID}, 1},
		{"by tag", store.EntryFilter{TagID: tag.ID}, 1},
		{"limit keeps total", store.EntryFilter{Limit: 1}, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			list, total, err := f.svc.List(f.ctx, c.filter)
			if err != nil {
				t.Fatal(err)
			}
			if total != c.want {
				t.Fatalf("total=%d, ожидали %d", total, c.want)
			}
			if c.filter.Limit == 1 && len(list) != 1 {
				t.Fatalf("limit не применён: %d", len(list))
			}
		})
	}
}

func TestDeleteCascades(t *testing.T) {
	f := newFixture(t)
	in := articleInput("Удаляемый")
	in.Tags = []string{"x"}
	e := f.create(t, in)
	if err := f.svc.Delete(f.ctx, f.author, e.ID); err != nil {
		t.Fatal(err)
	}
	var revs, links int
	f.db.QueryRow(`SELECT COUNT(*) FROM entry_revisions WHERE entry_id = ?`, e.ID).Scan(&revs)
	f.db.QueryRow(`SELECT COUNT(*) FROM entry_terms WHERE entry_id = ?`, e.ID).Scan(&links)
	if revs != 0 || links != 0 {
		t.Fatalf("остались ревизии (%d) или связи (%d)", revs, links)
	}
}

func TestTermCycleRejected(t *testing.T) {
	f := newFixture(t)
	a := f.category(t, "А", 0)
	b := f.category(t, "Б", a.ID)
	c := f.category(t, "В", b.ID)
	a.ParentID = c.ID
	if err := f.tax.Update(f.ctx, f.author, a); !errors.Is(err, ErrTermCycle) {
		t.Fatalf("цикл А→В→Б→А не обнаружен: %v", err)
	}
	b.ParentID = b.ID
	if err := f.tax.Update(f.ctx, f.author, b); !errors.Is(err, ErrTermCycle) {
		t.Fatalf("рубрика-родитель сама себе: %v", err)
	}
}

func TestDeleteTermUnlinksEntries(t *testing.T) {
	f := newFixture(t)
	cat := f.category(t, "Спорт", 0)
	in := articleInput("Матч")
	in.CategoryIDs = []int64{cat.ID}
	e := f.create(t, in)
	if err := f.tax.Delete(f.ctx, f.author, cat.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := f.svc.Get(f.ctx, e.ID)
	if len(got.Categories) != 0 {
		t.Fatalf("связь с удалённой рубрикой осталась: %v", got.Categories)
	}
}

func TestTagsDeduplicatedBySlug(t *testing.T) {
	f := newFixture(t)
	in := articleInput("Теги")
	in.Tags = []string{"Театр", "театр", " ", "Кино"}
	e := f.create(t, in)
	if len(e.Tags) != 2 {
		t.Fatalf("ожидали 2 тега, получили %v", e.Tags)
	}
}

type denyAll struct{}

func (denyAll) Can(access.Actor, string, access.Resource) bool { return false }

func TestServiceAsksAuthorizer(t *testing.T) {
	f := newFixture(t)
	e := f.create(t, articleInput("Т"))
	f.svc.Auth = denyAll{}
	f.tax.Auth = denyAll{}
	checks := map[string]error{}
	_, checks["create"] = f.svc.Create(f.ctx, f.author, "article", articleInput("X"))
	_, checks["update"] = f.svc.Update(f.ctx, f.author, e.ID, articleInput("X"))
	_, checks["transition"] = f.svc.Transition(f.ctx, f.author, e.ID, "submit")
	_, checks["rollback"] = f.svc.Rollback(f.ctx, f.author, e.ID, 1)
	checks["delete"] = f.svc.Delete(f.ctx, f.author, e.ID)
	checks["term"] = f.tax.Create(f.ctx, f.author, &Term{Vocabulary: VocabTag, Name: "x"})
	for op, err := range checks {
		if !errors.Is(err, ErrForbidden) {
			t.Errorf("%s: ожидали ErrForbidden, получили %v", op, err)
		}
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Привет, мир!":            "privet-mir",
		"  Щука и ёж  ":           "shchuka-i-ezh",
		"Go 1.25 — вышел":         "go-1-25-vyshel",
		"Объявление":              "obyavlenie",
		"!!!":                     "",
		"Already-latin_and-UPPER": "already-latin-and-upper",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, ожидали %q", in, got, want)
		}
	}
}
