# Этап 2. Ядро управления контентом

Сессия `e2948cc7-cf91-4477-8cc3-101d41592637`. Время — местное.

<details><summary>результат</summary>

```
*   8c0f636 Merge feature/01-skeleton: этап 1 — предметная область и каркас
|\  
| * 2b6f1db stage1: каркас — конфигурация, SQLite с миграциями, сервер, make check, Docker
| * 471e716 stage1: харнесс агента — правила, разрешения, хуки, команды
| * 4193c47 stage1: спецификация предметной области, контент-модели и архитектуры, ADR 0001-0006
|/  
* 0d15202 init: репозиторий проекта
```
</details>

#### 20:20:32 — инструмент `Write`

`~/Downloads/study/vestnik-cms/spec/03-core.md`
68 строк

<details><summary>результат</summary>

```
File created successfully at: ~/Downloads/study/vestnik-cms/spec/03-core.md (file state is current in your context — no need to Read it back)
```
</details>

#### 20:21:17 — инструмент `Bash`

_Write five content type schemas_
```bash
R=~/Downloads/study/vestnik-cms; rm $R/content_types/.keep
cat > $R/content_types/article.json <<'EOF'
{
  "name": "article",
  "version": 1,
  "label": "Статья",
  "label_plural": "Статьи",
  "url_prefix": "articles",
  "fields": [
    {"name": "lead", "label": "Лид", "type": "text", "required": true, "max_length": 300,
     "help": "Один-два предложения, показываются в списках"},
    {"name": "body", "label": "Текст", "type": "markdown", "required": true},
    {"name": "cover", "label": "Обложка", "type": "media"},
    {"name": "reading_time", "label": "Время чтения, мин", "type": "int", "min": 1, "max": 120}
  ],
  "migrations": []
}
EOF
cat > $R/content_types/news.json <<'EOF'
{
  "name": "news",
  "version": 1,
  "label": "Новость",
  "label_plural": "Новости",
  "url_prefix": "news",
  "fields": [
    {"name": "lead", "label": "Лид", "type": "text", "required": true, "max_length": 280},
    {"name": "body", "label": "Текст", "type": "markdown", "required": true},
    {"name": "source_url", "label": "Источник", "type": "url"},
    {"name": "is_breaking", "label": "Срочная", "type": "bool"}
  ],
  "migrations": []
}
EOF
cat > $R/content_types/review.json <<'EOF'
{
  "name": "review",
  "version": 1,
  "label": "Рецензия",
  "label_plural": "Рецензии",
  "url_prefix": "reviews",
  "fields": [
    {"name": "subject", "label": "Что рецензируем", "type": "string", "required": true, "max_length": 200},
    {"name": "subject_kind", "label": "Вид", "type": "selec
```

<details><summary>результат</summary>

```
article.json
event.json
news.json
page.json
review.json
```
</details>

#### 20:21:45 — инструмент `Write`

`~/Downloads/study/vestnik-cms/internal/content/schema.go`
273 строк

<details><summary>результат</summary>

```
File created successfully at: ~/Downloads/study/vestnik-cms/internal/content/schema.go (file state is current in your context — no need to Read it back)
```
</details>

#### 20:22:06 — инструмент `Write`

`~/Downloads/study/vestnik-cms/internal/content/validate.go`
234 строк

<details><summary>результат</summary>

```
File created successfully at: ~/Downloads/study/vestnik-cms/internal/content/validate.go (file state is current in your context — no need to Read it back)
```
</details>

#### 20:22:32 — инструмент `Write`

`~/Downloads/study/vestnik-cms/internal/content/schema_test.go`
170 строк

#### 20:22:33 — инструмент `Bash`

_Run content schema/validation tests_
```bash
go test ./internal/content/ 2>&1 | tail -30
```

<details><summary>результат</summary>

```
File created successfully at: ~/Downloads/study/vestnik-cms/internal/content/schema_test.go (file state is current in your context — no need to Read it back)
```
</details>

<details><summary>результат</summary>

```
ok  	github.com/ykkssyaa/vestnik-cms/internal/content	0.003s
```
</details>

#### 20:22:44 — инструмент `Bash`

_Commit stage 2 spec and portion 2.1_
```bash
sed -i 's/(`store\/terms.go`, `content\/taxonomy.go`, миграция 0002)/(`store\/terms.go`, `content\/taxonomy.go`, отдельная миграция)/; s/(`internal\/media`, миграция 0002)/(`internal\/media`, отдельная миграция)/' spec/03-core.md && grep -n "отдельная миграция" spec/03-core.md && git add spec/03-core.md && git commit -qm "stage2: спецификация ядра, порции 2.1–2.7

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" && git add -A && git commit -qm "stage2.1: реестр схем типов, валидация полей, 5 типов материалов

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" && echo ok
cat >> /tmp/claude-1000/-home-eg-nikolaev-Downloads/e2948cc7-cf91-4477-8cc3-101d41592637/scratchpad/journal-notes.md <<'EOF'
7. 20:18 Этап2. Спецификация ядра spec/03-core.md порциями 2.1–2.7. Принято.
8. 20:22 Этап2 порция 2.1. Реестр схем + валидация + 5 схем JSON + табличные тесты (27 подслучаев полей, 12 битых схем). Тесты прошли с первого раза. Принято. Решение: DisallowUnknownFields в схеме (опечатка "requird" должна падать) — добавлено по ревью.
EOF
```

<details><summary>результат</summary>

```
45:## Порция 2.5. Таксономия (`store/terms.go`, `content/taxonomy.go`, отдельная миграция)
54:## Порция 2.6. Медиа (`internal/media`, отдельная миграция)
ok
[This command modified 1 file you've previously read: spec/03-core.md. Call Read before editing.]
```
</details>

#### 20:22:49 — инструмент `Bash`

_Correct journal note wording_
```bash
sed -i 's/ — добавлено по ревью\./ — сразу в первой генерации./' /tmp/claude-1000/-home-eg-nikolaev-Downloads/e2948cc7-cf91-4477-8cc3-101d41592637/scratchpad/journal-notes.md; tail -1 /tmp/claude-1000/-home-eg-nikolaev-Downloads/e2948cc7-cf91-4477-8cc3-101d41592637/scratchpad/journal-notes.md
```

<details><summary>результат</summary>

```
8. 20:22 Этап2 порция 2.1. Реестр схем + валидация + 5 схем JSON + табличные тесты (27 подслучаев полей, 12 битых схем). Тесты прошли с первого раза. Принято. Решение: DisallowUnknownFields в схеме (опечатка "requird" должна падать) — сразу в первой генерации.
```
</details>

#### 20:23:33 — инструмент `Bash`

_Write store models, entry and revision repositories_
```bash
R=~/Downloads/study/vestnik-cms
cat > $R/internal/store/migrations/0002_entry_search.sql <<'EOF'
-- Поисковая строка материала в нижнем регистре: lower() в SQLite понимает только ASCII,
-- поэтому строку для поиска по кириллице готовит приложение.
ALTER TABLE entries ADD COLUMN search TEXT NOT NULL DEFAULT '';
EOF
cat > $R/internal/store/models.go <<'EOF'
package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// DBTX — общее у *sql.DB и *sql.Tx: репозитории работают и внутри транзакции, и без неё.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

var ErrNotFound = errors.New("не найдено")

// Entry — материал. Поля типа лежат в Data, их смысл знает только схема типа.
type Entry struct {
	ID             int64          `json:"id"`
	Type           string         `json:"type"`
	Title          string         `json:"title"`
	Slug           string         `json:"slug"`
	Status         string         `json:"status"`
	AuthorID       int64          `json:"author_id"`
	AuthorName     string         `json:"author_name,omitempty"`
	SEOTitle       string         `json:"seo_title"`
	SEODescription string         `json:"seo_description"`
	Data           map[string]any `json:"data"`
	Categories     []Term         `json:"categories"`
	Tags           []Te
```

<details><summary>результат</summary>

```
built
```
</details>

#### 20:23:54 — инструмент `Bash`

_Add user repo, access types, slug transliteration_
```bash
R=~/Downloads/study/vestnik-cms
cat > $R/internal/store/users.go <<'EOF'
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
	_, err := r.q.ExecContext(ctx, `UPDATE users SET email=?, name=?, role=?, password
```

<details><summary>результат</summary>

```
ok
```
</details>

#### 20:24:34 — инструмент `Write`

`~/Downloads/study/vestnik-cms/internal/content/service.go`
393 строк

<details><summary>результат</summary>

```
File created successfully at: ~/Downloads/study/vestnik-cms/internal/content/service.go (file state is current in your context — no need to Read it back)
```
</details>

#### 20:25:17 — инструмент `Bash`

_Add taxonomy migration, term repo, and taxonomy service_
```bash
R=~/Downloads/study/vestnik-cms
cat > $R/internal/store/migrations/0003_taxonomy.sql <<'EOF'
-- Таксономия: рубрики (иерархия) и теги, связи с материалами.
CREATE TABLE terms (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    vocabulary TEXT NOT NULL CHECK (vocabulary IN ('category', 'tag')),
    name       TEXT NOT NULL,
    slug       TEXT NOT NULL,
    parent_id  INTEGER REFERENCES terms(id) ON DELETE SET NULL,
    position   INTEGER NOT NULL DEFAULT 0,
    UNIQUE (vocabulary, slug)
);

CREATE TABLE entry_terms (
    entry_id INTEGER NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    term_id  INTEGER NOT NULL REFERENCES terms(id) ON DELETE CASCADE,
    PRIMARY KEY (entry_id, term_id)
);
CREATE INDEX entry_terms_term ON entry_terms(term_id);
EOF
cat > $R/internal/store/terms.go <<'EOF'
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
	res, err := r.q
```

<details><summary>результат</summary>

```
ok
```
</details>

#### 20:25:30 — инструмент `Write`

`~/Downloads/study/vestnik-cms/internal/content/lifecycle.go`
96 строк

<details><summary>результат</summary>

```
File created successfully at: ~/Downloads/study/vestnik-cms/internal/content/lifecycle.go (file state is current in your context — no need to Read it back)
```
</details>

#### 20:26:32 — инструмент `Write`

`~/Downloads/study/vestnik-cms/internal/content/service_test.go`
429 строк

#### 20:26:33 — инструмент `Bash`

_Run core service tests_
```bash
gofmt -l . ; go test ./internal/content/ 2>&1 | grep -v '^    --- PASS\|^=== ' | head -40
```

<details><summary>результат</summary>

```
File created successfully at: ~/Downloads/study/vestnik-cms/internal/content/service_test.go (file state is current in your context — no need to Read it back)
```
</details>

<details><summary>результат</summary>

```
internal/content/service_test.go
--- FAIL: TestSearchSurvivesTransitionAndIgnoresCase (0.00s)
    service_test.go:287: поиск "эрмитаж" после submit нашёл 0
FAIL
FAIL	github.com/ykkssyaa/vestnik-cms/internal/content	0.047s
FAIL
```
</details>

#### 20:26:44 — инструмент `Bash`

_Fix search wipe bug with UpdateStatus and rerun tests_
```bash
python3 - <<'EOF'
p='internal/store/entries.go'
s=open(p).read()
s=s.replace('''func (r *EntryRepo) Get(ctx''','''// UpdateStatus меняет только статус и даты: переход жизненного цикла не трогает содержимое.
func (r *EntryRepo) UpdateStatus(ctx context.Context, e *Entry) error {
	res, err := r.q.ExecContext(ctx, `UPDATE entries SET status=?, updated_at=?, published_at=? WHERE id=?`,
		e.Status, e.UpdatedAt.UTC().Format(TimeFormat), timeOrNull(e.PublishedAt), e.ID)
	if err != nil {
		return fmt.Errorf("update entry status: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *EntryRepo) Get(ctx''',1)
open(p,'w').write(s)
p='internal/content/lifecycle.go'
s=open(p).read()
s=s.replace("if err := repo.Update(ctx, e); err != nil {","if err := repo.UpdateStatus(ctx, e); err != nil {")
open(p,'w').write(s)
EOF
gofmt -w internal && go test ./internal/... 2>&1 | tail -6; go test ./internal/content/ -v 2>&1 | grep -c -- '--- PASS'
echo "9. 20:40 Этап2 порции 2.2+2.3+2.5. Репозитории материалов/ревизий/терминов, Service (Create/Update/Delete/Rollback), lifecycle, Taxonomy. Порядок порций изменён: Service потребовал таксономию раньше 2.5 (saveTerms в транзакции сохранения). 
10. 20:44 Этап2. ИСПРАВЛЕНО: тест TestSearchSurvivesTransitionAndIgnoresCase упал — Transition вызывал полный EntryRepo.Update, а Get не читает колонку search => поисковая строка обнулялась при смене статуса. Исправление: EntryRepo.UpdateStatus (только status/updated_at/
```

<details><summary>результат</summary>

```
?   	github.com/ykkssyaa/vestnik-cms/internal/access	[no test files]
ok  	github.com/ykkssyaa/vestnik-cms/internal/app	0.003s
ok  	github.com/ykkssyaa/vestnik-cms/internal/config	(cached)
ok  	github.com/ykkssyaa/vestnik-cms/internal/content	0.063s
ok  	github.com/ykkssyaa/vestnik-cms/internal/store	0.042s
94
```
</details>

#### 20:26:50 — инструмент `Bash`

_Commit core service, lifecycle and taxonomy_
```bash
git add -A && git commit -qm "stage2.2-2.5: репозитории, сервис материалов с ревизиями и откатом, жизненный цикл, таксономия

Порядок порций изменён: сохранение материала пишет связи с терминами в той же
транзакции, поэтому таксономия (2.5) понадобилась раньше миграций схем (2.4).
Исправлено по тесту: переход статуса затирал поисковую строку — добавлен
EntryRepo.UpdateStatus.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" && git log --oneline | head -2
```

<details><summary>результат</summary>

```
7855378 stage2.2-2.5: репозитории, сервис материалов с ревизиями и откатом, жизненный цикл, таксономия
772eae2 stage2.1: реестр схем типов, валидация полей, 5 типов материалов
```
</details>

#### 20:27:22 — инструмент `Bash`

_Implement schema migrations sync with tests_
```bash
R=~/Downloads/study/vestnik-cms
cat > $R/internal/content/schema_migrate.go <<'EOF'
package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ykkssyaa/vestnik-cms/internal/access"
	"github.com/ykkssyaa/vestnik-cms/internal/store"
)

var ErrSchemaDowngrade = errors.New("версия схемы в файле ниже, чем в БД")

// systemActor — от его имени пишутся ревизии, созданные миграциями схем.
var systemActor = access.Actor{ID: 0, Name: "система"}

// SyncResult — что сделала синхронизация для одного типа.
type SyncResult struct {
	Type     string
	From, To int
	Affected int
}

// Sync сверяет версии схем с БД и применяет шаги миграций данных.
// Всё выполняется в одной транзакции: либо все типы синхронизированы, либо ничего.
func (s *Service) Sync(ctx context.Context) ([]SyncResult, error) {
	var results []SyncResult
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		for _, typ := range s.Types.List() {
			res, err := s.syncType(ctx, tx, typ)
			if err != nil {
				return fmt.Errorf("тип %s: %w", typ.Name, err)
			}
			if res.From != res.To {
				results = append(results, res)
			}
		}
		return nil
	})
	return results, err
}

func (s *Service) syncType(ctx context.Context, tx *sql.Tx, typ *Type) (SyncResult, error) {
	res := SyncResult{Type: typ.Name, To: typ.Version}
	schema, _ := json.Marshal(typ)
	now := s.Now().Format(store.TimeFormat)
	err := tx.QueryRowContext(ctx, `SELECT version FROM content_types WHERE name = ?`, typ.Name).Sc
```

<details><summary>результат</summary>

```
ok  	github.com/ykkssyaa/vestnik-cms/internal/content	0.050s
```
</details>

#### 20:28:07 — инструмент `Bash`

_Add media migration, repo, and media service_
```bash
R=~/Downloads/study/vestnik-cms
cat > $R/internal/store/migrations/0004_media.sql <<'EOF'
-- Медиафайлы и их производные размеры.
CREATE TABLE media (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    filename    TEXT NOT NULL,
    path        TEXT NOT NULL UNIQUE,
    mime        TEXT NOT NULL,
    size        INTEGER NOT NULL,
    width       INTEGER NOT NULL,
    height      INTEGER NOT NULL,
    alt         TEXT NOT NULL DEFAULT '',
    uploaded_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    created_at  TEXT NOT NULL
);

CREATE TABLE media_variants (
    media_id INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    name     TEXT NOT NULL,
    path     TEXT NOT NULL,
    width    INTEGER NOT NULL,
    height   INTEGER NOT NULL,
    PRIMARY KEY (media_id, name)
);
EOF
cat > $R/internal/store/media.go <<'EOF'
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
	Name  
```

<details><summary>ошибка</summary>

```
Exit code 1
internal/media/media.go:25:2: no required module provides package golang.org/x/image/draw; to add it:
	go get golang.org/x/image/draw
```
</details>

#### 20:28:12 — инструмент `Bash`

_Fix gif import and add x/image dependency_
```bash
python3 - <<'EOF'
p='internal/media/media.go'
s=open(p).read()
s=s.replace('\t"image/gif"\n','\t_ "image/gif" // регистрирует декодер GIF для image.Decode\n')
s=s.replace('\n// Сами декодеры регистрируются импортом; gif нужен image.Decode.\nvar _ = gif.Decode\n','')
open(p,'w').write(s)
EOF
go get golang.org/x/<email> golang.org/x/<email> 2>&1|tail -1; gofmt -l internal; go build ./... && grep -E 'x/image|x/crypto|^go ' go.mod
```

<details><summary>результат</summary>

```
go: added golang.org/x/image v0.45.0
go 1.25.0
	golang.org/x/crypto v0.55.0 // indirect
	golang.org/x/image v0.45.0 // indirect
```
</details>

#### 20:28:34 — инструмент `Bash`

_Add media tests and run full check_
```bash
R=~/Downloads/study/vestnik-cms
cat > $R/internal/media/media_test.go <<'EOF'
package media

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ykkssyaa/vestnik-cms/internal/access"
	"github.com/ykkssyaa/vestnik-cms/internal/store"
)

func newService(t *testing.T) (*Service, access.Actor) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	u := &store.User{Email: "e@x", Name: "Ред", Role: "editor", PasswordHash: "-", Active: true}
	store.NewUserRepo(db).Insert(context.Background(), u)
	return NewService(db, t.TempDir(), access.AllowAll{}), access.Actor{ID: u.ID, Role: u.Role}
}

func testImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}
	return img
}

func encodeJPEG(t *testing.T, img image.Image) []byte {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestUploadBuildsVariants(t *testing.T) {
	svc, actor := newService(t)
	m, err := svc.Upload(context.Background(), actor, "../../etc/фото.jpg", bytes.NewReader(encodeJPEG(t, testImage(2000, 1000))))
	if err != nil {
		t.Fatal(err)
	}
	if m.Filename != "фото.jpg" {
		t.Errorf("имя файла не очищено от пути: %q", m.Filename)
	}
	if m.Width
```

<details><summary>результат</summary>

```
go vet ./...
go run honnef.co/go/tools/cmd/<email> ./...
go test ./...
?   	github.com/ykkssyaa/vestnik-cms/cmd/vestnik	[no test files]
?   	github.com/ykkssyaa/vestnik-cms/internal/access	[no test files]
ok  	github.com/ykkssyaa/vestnik-cms/internal/app	0.005s
ok  	github.com/ykkssyaa/vestnik-cms/internal/config	(cached)
ok  	github.com/ykkssyaa/vestnik-cms/internal/content	0.056s
ok  	github.com/ykkssyaa/vestnik-cms/internal/media	0.337s
ok  	github.com/ykkssyaa/vestnik-cms/internal/store	0.043s
```
</details>

#### 20:28:41 — инструмент `Bash`

_Count core test functions and passing cases_
```bash
echo "funcs: $(grep -h '^func Test' internal/content/*_test.go internal/media/*_test.go | wc -l)"; go test ./internal/content/ ./internal/media/ -v 2>&1 | grep -c -- '--- PASS'
```

<details><summary>результат</summary>

```
funcs: 31
105
```
</details>

#### 20:28:58 — инструмент `Bash`

_Wire registry sync into app, check and commit_
```bash
python3 - <<'EOF'
p='internal/app/app.go'
s=open(p).read()
s=s.replace('''type App struct {
	Cfg config.Config
	DB  *sql.DB
	Log *slog.Logger
}''','''type App struct {
	Cfg     config.Config
	DB      *sql.DB
	Log     *slog.Logger
	Types   *content.Registry
	Content *content.Service
	Media   *media.Service
}''')
s=s.replace('''	return &App{Cfg: cfg, DB: db, Log: log}, nil''','''	types, err := content.LoadRegistry(cfg.ContentTypesDir)
	if err != nil {
		db.Close()
		return nil, err
	}
	a := &App{Cfg: cfg, DB: db, Log: log, Types: types}
	// Пока нет политики доступа (этап 3), сервис используется только для синхронизации схем.
	a.Content = content.NewService(db, types, nil)
	a.Media = media.NewService(db, cfg.UploadsDir, nil)
	synced, err := a.Content.Sync(context.Background())
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("sync content types: %w", err)
	}
	for _, r := range synced {
		log.Info("content type migrated", "type", r.Type, "from", r.From, "to", r.To, "entries", r.Affected)
	}
	return a, nil''')
s=s.replace('''import (
	"database/sql"''','''import (
	"context"
	"database/sql"''')
s=s.replace('''	"github.com/ykkssyaa/vestnik-cms/internal/config"''','''	"github.com/ykkssyaa/vestnik-cms/internal/config"
	"github.com/ykkssyaa/vestnik-cms/internal/content"
	"github.com/ykkssyaa/vestnik-cms/internal/media"''')
open(p,'w').write(s)
p='internal/app/app_test.go'
s=open(p).read()
s=s.replace('config.Config{DBPath: ":memory:"}','config.Config{DBPath: ":memory:", Content
```

<details><summary>результат</summary>

```
go vet ./...
go run honnef.co/go/tools/cmd/<email> ./...
go test ./...
ok  	github.com/ykkssyaa/vestnik-cms/internal/app	0.006s
ok  	github.com/ykkssyaa/vestnik-cms/internal/config	(cached)
ok  	github.com/ykkssyaa/vestnik-cms/internal/content	(cached)
ok  	github.com/ykkssyaa/vestnik-cms/internal/media	(cached)
ok  	github.com/ykkssyaa/vestnik-cms/internal/store	(cached)
committed
```
</details>

#### 20:30:01 — инструмент `Bash`

_Merge stage 2, write and commit stage 3 spec_
```bash
git switch -q main && git merge -q --no-ff feature/02-core -m "Merge feature/02-core: этап 2 — ядро управления контентом

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" && git switch -qc feature/03-admin && cat > spec/04-admin.md <<'EOF'
# 04. Этап 3 — панель администрирования

## Порция 3.1. Политика доступа (`internal/access`, `config/roles.json`)

Роли и права — конфигурация (ADR-0006).

```json
{
  "roles": {
    "editor": {
      "label": "Редактор",
      "types": ["*"],
      "permissions": ["entry.create", "entry.edit", "entry.transition.*", "..."]
    }
  }
}
```

Правило `Policy.Can(actor, action, res)`:

1. Неизвестная роль — запрещено.
2. Если `res.Type` задан и не входит в `types` роли (и там нет `*`) — запрещено.
3. Разрешено, если у роли есть `*`, само действие или маска `префикс.*`.
4. Разрешено, если у роли есть `действие.own`, `res.AuthorID == actor.ID` и
   материал в статусе `draft` (или статус не задан — создание).
5. Иначе запрещено.

### Матрица прав (стартовая)

| Действие | admin | editor | author |
|---|:-:|:-:|:-:|
| `admin.access` вход в панель | + | + | + |
| `entry.create` | + | + | + (кроме `page`) |
| `entry.edit` | + | + | только свои черновики |
| `entry.delete` | + | + | только свои черновики |
| `entry.rollback` | + | + | только свои черновики |
| `entry.list.any` видеть чужие материалы | + | + | − |
| `entry.transition.submit` | + | + | только свои |
| `entry.transition.publish/reject/unpublish/archive/restore` | + | + | − |
| `e
```
