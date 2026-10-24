package admin_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/ykkssyaa/vestnik-cms/internal/testkit"
)

var articleForm = url.Values{
	"title": {"Мост откроют в ноябре"}, "data.lead": {"Коротко о мосте"},
	"data.body": {"Текст про **мост**."}, "data.reading_time": {"3"}, "tags": {"мосты, транспорт"},
}

func TestAnonymousIsRedirectedToLogin(t *testing.T) {
	e := testkit.NewEnv(t, "../../..")
	c := e.Anon()
	for _, path := range []string{"/admin/", "/admin/entries", "/admin/entries/1", "/admin/users", "/admin/audit", "/admin/media"} {
		resp, _ := c.Get(path)
		if resp.StatusCode != http.StatusSeeOther || !strings.HasPrefix(resp.Header.Get("Location"), "/admin/login") {
			t.Errorf("%s: %d %s", path, resp.StatusCode, resp.Header.Get("Location"))
		}
	}
}

func TestWrongPasswordAndOpenRedirect(t *testing.T) {
	e := testkit.NewEnv(t, "../../..")
	c := e.Anon()
	resp, body := c.PostBody("/admin/login", url.Values{"email": {"admin@v.ru"}, "password": {"nope"}})
	if resp.StatusCode != http.StatusUnauthorized || !strings.Contains(body, "Неверный email или пароль") {
		t.Fatalf("неверный пароль: %d", resp.StatusCode)
	}
	for _, next := range []string{"//evil.example/", "https://evil.example/", "/\\evil.example"} {
		resp := e.Anon().Post("/admin/login", url.Values{"email": {"admin@v.ru"}, "password": {"password1"}, "next": {next}})
		if loc := resp.Header.Get("Location"); loc != "/admin/" {
			t.Errorf("next=%q: открытый редирект на %q", next, loc)
		}
	}
}

func TestPostWithoutCSRFIsRejected(t *testing.T) {
	e := testkit.NewEnv(t, "../../..")
	c := e.Login("editor@v.ru")
	resp := c.Post("/admin/terms", url.Values{"csrf": {"forged"}, "vocabulary": {"category"}, "name": {"Взлом"}})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("POST с чужим CSRF: %d", resp.StatusCode)
	}
	terms, _ := e.App.Taxonomy.List(context.Background(), "category")
	if len(terms) != 0 {
		t.Fatal("рубрика создана без CSRF-токена")
	}
}

// Роль → страница/действие → ожидаемый код ответа.
func TestAdminAccessByRole(t *testing.T) {
	e := testkit.NewEnv(t, "../../..")
	author := e.Login("author@v.ru")
	resp := author.Post("/admin/entries/new?type=article", testkit.Clone(articleForm))
	foreignID := testkit.EntryID(t, resp)

	cases := []struct {
		who, method, path string
		want              int
	}{
		{"author@v.ru", "GET", "/admin/users", 403},
		{"author@v.ru", "GET", "/admin/audit", 403},
		{"author@v.ru", "GET", "/admin/terms", 403},
		{"author@v.ru", "GET", "/admin/entries/new?type=page", 403},
		{"author@v.ru", "GET", "/admin/entries/new?type=news", 200},
		{"author2@v.ru", "GET", "/admin/entries/" + foreignID, 403},
		{"author2@v.ru", "POST", "/admin/entries/" + foreignID + "/delete", 403},
		{"author2@v.ru", "POST", "/admin/entries/batch", 403},
		{"editor@v.ru", "GET", "/admin/entries/" + foreignID, 200},
		{"editor@v.ru", "GET", "/admin/users", 403},
		{"editor@v.ru", "GET", "/admin/audit", 200},
		{"admin@v.ru", "GET", "/admin/users", 200},
	}
	for _, c := range cases {
		t.Run(c.who+" "+c.method+" "+c.path, func(t *testing.T) {
			cl := e.Login(c.who)
			var resp *http.Response
			if c.method == "GET" {
				resp, _ = cl.Get(c.path)
			} else {
				resp = cl.Post(c.path, url.Values{"action": {"publish"}, "ids": {foreignID}})
			}
			if resp.StatusCode != c.want {
				t.Fatalf("код %d, ожидали %d", resp.StatusCode, c.want)
			}
		})
	}
}

func TestAuthorSeesOnlyOwnEntries(t *testing.T) {
	e := testkit.NewEnv(t, "../../..")
	a1, a2 := e.Login("author@v.ru"), e.Login("author2@v.ru")
	a1.Post("/admin/entries/new?type=article", testkit.Clone(articleForm))
	other := testkit.Clone(articleForm)
	other.Set("title", "Чужой секретный черновик")
	a2.Post("/admin/entries/new?type=article", other)
	_, body := a1.Get("/admin/entries")
	if strings.Contains(body, "Чужой секретный черновик") {
		t.Fatal("автор видит чужой черновик в списке")
	}
	_, body = a1.Get("/admin/entries?author=0")
	if strings.Contains(body, "Чужой секретный черновик") {
		t.Fatal("фильтр по автору обходит ограничение")
	}
}

func TestEntryFormValidationShowsErrors(t *testing.T) {
	e := testkit.NewEnv(t, "../../..")
	c := e.Login("author@v.ru")
	bad := testkit.Clone(articleForm)
	bad.Set("data.reading_time", "999")
	bad.Del("data.body")
	resp, body := c.PostBody("/admin/entries/new?type=article", bad)
	if resp.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(body, "не больше 120") || !strings.Contains(body, "обязательное поле") {
		t.Fatalf("ошибки валидации не показаны: %d", resp.StatusCode)
	}
	if !strings.Contains(body, "Мост откроют в ноябре") {
		t.Fatal("введённые данные потеряны при ошибке")
	}
}

func TestEntryFormForEveryType(t *testing.T) {
	e := testkit.NewEnv(t, "../../..")
	c := e.Login("admin@v.ru")
	for _, typ := range e.App.Types.List() {
		resp, body := c.Get("/admin/entries/new?type=" + typ.Name)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: %d", typ.Name, resp.StatusCode)
		}
		for _, f := range typ.Fields {
			if !strings.Contains(body, `name="data.`+f.Name+`"`) {
				t.Errorf("%s: в форме нет поля %s", typ.Name, f.Name)
			}
		}
	}
}

// Сквозной сценарий администратора (spec/04-admin.md, 3.5).
func TestAdminScenario(t *testing.T) {
	e := testkit.NewEnv(t, "../../..")
	adm := e.Login("admin@v.ru")

	// 1. Администратор заводит нового автора.
	resp := adm.Post("/admin/users", url.Values{"email": {"new@v.ru"}, "name": {"Новый автор"}, "role": {"author"}, "password": {"password1"}, "active": {"on"}})
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("создание пользователя: %d", resp.StatusCode)
	}
	// 2. Администратор создаёт рубрику.
	adm.Post("/admin/terms", url.Values{"vocabulary": {"category"}, "name": {"Город"}})
	cats, _ := e.App.Taxonomy.List(context.Background(), "category")
	if len(cats) != 1 {
		t.Fatal("рубрика не создана")
	}

	// 3. Новый автор пишет два материала и отправляет их на модерацию.
	author := e.Login("new@v.ru")
	var ids []string
	for _, title := range []string{"Первый", "Второй"} {
		f := testkit.Clone(articleForm)
		f.Set("title", title)
		f.Set("category_ids", "1")
		id := testkit.EntryID(t, author.Post("/admin/entries/new?type=article", f))
		author.Post("/admin/entries/"+id+"/transition", url.Values{"name": {"submit"}})
		ids = append(ids, id)
	}
	// Автор не может опубликовать сам.
	if resp := author.Post("/admin/entries/"+ids[0]+"/transition", url.Values{"name": {"publish"}}); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("автор опубликовал сам: %d", resp.StatusCode)
	}

	// 4. Редактор видит очередь модерации и публикует пакетно.
	ed := e.Login("editor@v.ru")
	_, body := ed.Get("/admin/entries?status=review")
	if !strings.Contains(body, "Первый") || !strings.Contains(body, "Второй") {
		t.Fatal("материалов нет в очереди модерации")
	}
	resp = ed.Post("/admin/entries/batch", url.Values{"action": {"publish"}, "ids": ids, "back": {"/admin/entries?status=review"}})
	if resp.StatusCode != http.StatusSeeOther || !strings.Contains(testkit.Flash(resp), "Выполнено: 2") {
		t.Fatalf("пакетная публикация: %d %q", resp.StatusCode, testkit.Flash(resp))
	}
	// Повторная публикация — оба пропущены как недопустимый переход.
	resp = ed.Post("/admin/entries/batch", url.Values{"action": {"publish"}, "ids": ids})
	if msg := testkit.Flash(resp); !strings.Contains(msg, "Выполнено: 0") || !strings.Contains(msg, "недопустимый переход): 2") {
		t.Fatalf("повторная публикация: %q", msg)
	}

	// 5. Всё видно в журнале действий.
	_, body = adm.Get("/admin/audit?action=entry.publish")
	if strings.Count(body, "entry.publish") < 3 { // два в таблице + опция фильтра
		t.Fatalf("публикации не попали в журнал")
	}
	_, body = adm.Get("/admin/audit?action=user.")
	if !strings.Contains(body, "new@v.ru") {
		t.Fatal("создание пользователя не попало в журнал")
	}
}

func TestRevisionsAndRollbackViaUI(t *testing.T) {
	e := testkit.NewEnv(t, "../../..")
	c := e.Login("editor@v.ru")
	id := testkit.EntryID(t, c.Post("/admin/entries/new?type=article", testkit.Clone(articleForm)))
	changed := testkit.Clone(articleForm)
	changed.Set("title", "Изменённый заголовок")
	c.Post("/admin/entries/"+id, changed)
	_, body := c.Get("/admin/entries/" + id + "/revisions")
	if !strings.Contains(body, "Ревизия 2") || !strings.Contains(body, "Ревизия 1") {
		t.Fatal("ревизии не показаны")
	}
	c.Post("/admin/entries/"+id+"/rollback", url.Values{"number": {"1"}})
	_, body = c.Get("/admin/entries/" + id)
	if !strings.Contains(body, `value="Мост откроют в ноябре"`) {
		t.Fatal("откат через интерфейс не восстановил заголовок")
	}
}

// Каждая страница панели рендерится без ошибок шаблона (ошибка шаблона = 500).
func TestAllPagesRender(t *testing.T) {
	e := testkit.NewEnv(t, "../../..")
	c := e.Login("admin@v.ru")
	id := testkit.EntryID(t, c.Post("/admin/entries/new?type=article", testkit.Clone(articleForm)))
	c.Post("/admin/terms", url.Values{"vocabulary": {"category"}, "name": {"Город"}})
	pages := []string{"/admin/", "/admin/entries", "/admin/entries?status=review&q=мост&page=2",
		"/admin/entries/" + id, "/admin/entries/" + id + "/revisions", "/admin/terms", "/admin/terms?edit=1",
		"/admin/media", "/admin/users", "/admin/users?edit=1", "/admin/audit", "/admin/audit?action=entry.",
		"/admin/entries/999"}
	for _, typ := range e.App.Types.List() {
		pages = append(pages, "/admin/entries/new?type="+typ.Name)
	}
	for _, p := range pages {
		resp, body := c.Get(p)
		if resp.StatusCode >= 500 || (resp.StatusCode != 200 && !strings.HasSuffix(p, "999")) {
			t.Errorf("%s: %d %s", p, resp.StatusCode, body[:min(len(body), 200)])
		}
	}
	if resp, _ := e.Anon().Get("/admin/login"); resp.StatusCode != 200 {
		t.Errorf("/admin/login: %d", resp.StatusCode)
	}
}
