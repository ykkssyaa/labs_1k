package admin_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/ykkssyaa/vestnik-cms/internal/access"
	"github.com/ykkssyaa/vestnik-cms/internal/app"
	"github.com/ykkssyaa/vestnik-cms/internal/auth"
	"github.com/ykkssyaa/vestnik-cms/internal/store"
)

type env struct {
	t   *testing.T
	app *app.App
	srv *httptest.Server
}

func newEnv(t *testing.T) *env {
	t.Helper()
	cfg := app.TestConfig("../../..")
	cfg.UploadsDir = t.TempDir()
	a, err := app.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(a.Handler())
	t.Cleanup(func() { srv.Close(); a.Close() })
	e := &env{t: t, app: a, srv: srv}
	e.user("admin@v.ru", "Админ", "admin")
	e.user("editor@v.ru", "Редактор", "editor")
	e.user("author@v.ru", "Автор", "author")
	e.user("author2@v.ru", "Другой автор", "author")
	return e
}

func (e *env) user(email, name, role string) *store.User {
	u, err := e.app.Auth.CreateUser(context.Background(), access.Actor{},
		auth.UserInput{Email: email, Name: name, Role: role, Password: "password1"})
	if err != nil {
		e.t.Fatal(err)
	}
	return u
}

// client — браузер одного пользователя: cookie и CSRF-токен.
type client struct {
	e    *env
	http *http.Client
	csrf string
}

func (e *env) anon() *client {
	jar, _ := cookiejar.New(nil)
	return &client{e: e, http: &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse // редиректы проверяем сами
	}}}
}

func (e *env) login(email string) *client {
	c := e.anon()
	resp := c.post("/admin/login", url.Values{"email": {email}, "password": {"password1"}})
	if resp.StatusCode != http.StatusSeeOther {
		e.t.Fatalf("вход %s: %d", email, resp.StatusCode)
	}
	c.get("/admin/") // получить CSRF-токен со страницы
	return c
}

var csrfRe = regexp.MustCompile(`name="csrf" value="([^"]+)"`)

func (c *client) do(req *http.Request) (*http.Response, string) {
	c.e.t.Helper()
	resp, err := c.http.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if m := csrfRe.FindSubmatch(body); m != nil {
		c.csrf = string(m[1])
	}
	return resp, string(body)
}

func (c *client) get(path string) (*http.Response, string) {
	req, _ := http.NewRequest(http.MethodGet, c.e.srv.URL+path, nil)
	return c.do(req)
}

func (c *client) post(path string, form url.Values) *http.Response {
	resp, _ := c.postBody(path, form)
	return resp
}

func (c *client) postBody(path string, form url.Values) (*http.Response, string) {
	if form.Get("csrf") == "" && c.csrf != "" {
		form.Set("csrf", c.csrf)
	}
	req, _ := http.NewRequest(http.MethodPost, c.e.srv.URL+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.do(req)
}

var articleForm = url.Values{
	"title": {"Мост откроют в ноябре"}, "data.lead": {"Коротко о мосте"},
	"data.body": {"Текст про **мост**."}, "data.reading_time": {"3"}, "tags": {"мосты, транспорт"},
}

func clone(v url.Values) url.Values {
	out := url.Values{}
	for k, vs := range v {
		out[k] = append([]string(nil), vs...)
	}
	return out
}

func entryIDFromLocation(t *testing.T, resp *http.Response) string {
	t.Helper()
	loc := resp.Header.Get("Location")
	m := regexp.MustCompile(`^/admin/entries/(\d+)$`).FindStringSubmatch(loc)
	if m == nil {
		t.Fatalf("ожидали редирект на материал, получили %d %q", resp.StatusCode, loc)
	}
	return m[1]
}

func TestAnonymousIsRedirectedToLogin(t *testing.T) {
	e := newEnv(t)
	c := e.anon()
	for _, path := range []string{"/admin/", "/admin/entries", "/admin/entries/1", "/admin/users", "/admin/audit", "/admin/media"} {
		resp, _ := c.get(path)
		if resp.StatusCode != http.StatusSeeOther || !strings.HasPrefix(resp.Header.Get("Location"), "/admin/login") {
			t.Errorf("%s: %d %s", path, resp.StatusCode, resp.Header.Get("Location"))
		}
	}
}

func TestWrongPasswordAndOpenRedirect(t *testing.T) {
	e := newEnv(t)
	c := e.anon()
	resp, body := c.postBody("/admin/login", url.Values{"email": {"admin@v.ru"}, "password": {"nope"}})
	if resp.StatusCode != http.StatusUnauthorized || !strings.Contains(body, "Неверный email или пароль") {
		t.Fatalf("неверный пароль: %d", resp.StatusCode)
	}
	for _, next := range []string{"//evil.example/", "https://evil.example/", "/\\evil.example"} {
		resp := e.anon().post("/admin/login", url.Values{"email": {"admin@v.ru"}, "password": {"password1"}, "next": {next}})
		if loc := resp.Header.Get("Location"); loc != "/admin/" {
			t.Errorf("next=%q: открытый редирект на %q", next, loc)
		}
	}
}

func TestPostWithoutCSRFIsRejected(t *testing.T) {
	e := newEnv(t)
	c := e.login("editor@v.ru")
	resp := c.post("/admin/terms", url.Values{"csrf": {"forged"}, "vocabulary": {"category"}, "name": {"Взлом"}})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("POST с чужим CSRF: %d", resp.StatusCode)
	}
	terms, _ := e.app.Taxonomy.List(context.Background(), "category")
	if len(terms) != 0 {
		t.Fatal("рубрика создана без CSRF-токена")
	}
}

// Роль → страница/действие → ожидаемый код ответа.
func TestAdminAccessByRole(t *testing.T) {
	e := newEnv(t)
	author := e.login("author@v.ru")
	resp := author.post("/admin/entries/new?type=article", clone(articleForm))
	foreignID := entryIDFromLocation(t, resp)

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
			cl := e.login(c.who)
			var resp *http.Response
			if c.method == "GET" {
				resp, _ = cl.get(c.path)
			} else {
				resp = cl.post(c.path, url.Values{"action": {"publish"}, "ids": {foreignID}})
			}
			if resp.StatusCode != c.want {
				t.Fatalf("код %d, ожидали %d", resp.StatusCode, c.want)
			}
		})
	}
}

func TestAuthorSeesOnlyOwnEntries(t *testing.T) {
	e := newEnv(t)
	a1, a2 := e.login("author@v.ru"), e.login("author2@v.ru")
	a1.post("/admin/entries/new?type=article", clone(articleForm))
	other := clone(articleForm)
	other.Set("title", "Чужой секретный черновик")
	a2.post("/admin/entries/new?type=article", other)
	_, body := a1.get("/admin/entries")
	if strings.Contains(body, "Чужой секретный черновик") {
		t.Fatal("автор видит чужой черновик в списке")
	}
	_, body = a1.get("/admin/entries?author=0")
	if strings.Contains(body, "Чужой секретный черновик") {
		t.Fatal("фильтр по автору обходит ограничение")
	}
}

func TestEntryFormValidationShowsErrors(t *testing.T) {
	e := newEnv(t)
	c := e.login("author@v.ru")
	bad := clone(articleForm)
	bad.Set("data.reading_time", "999")
	bad.Del("data.body")
	resp, body := c.postBody("/admin/entries/new?type=article", bad)
	if resp.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(body, "не больше 120") || !strings.Contains(body, "обязательное поле") {
		t.Fatalf("ошибки валидации не показаны: %d", resp.StatusCode)
	}
	if !strings.Contains(body, "Мост откроют в ноябре") {
		t.Fatal("введённые данные потеряны при ошибке")
	}
}

func TestEntryFormForEveryType(t *testing.T) {
	e := newEnv(t)
	c := e.login("admin@v.ru")
	for _, typ := range e.app.Types.List() {
		resp, body := c.get("/admin/entries/new?type=" + typ.Name)
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
	e := newEnv(t)
	adm := e.login("admin@v.ru")

	// 1. Администратор заводит нового автора.
	resp := adm.post("/admin/users", url.Values{"email": {"new@v.ru"}, "name": {"Новый автор"}, "role": {"author"}, "password": {"password1"}, "active": {"on"}})
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("создание пользователя: %d", resp.StatusCode)
	}
	// 2. Администратор создаёт рубрику.
	adm.post("/admin/terms", url.Values{"vocabulary": {"category"}, "name": {"Город"}})
	cats, _ := e.app.Taxonomy.List(context.Background(), "category")
	if len(cats) != 1 {
		t.Fatal("рубрика не создана")
	}

	// 3. Новый автор пишет два материала и отправляет их на модерацию.
	author := e.login("new@v.ru")
	var ids []string
	for _, title := range []string{"Первый", "Второй"} {
		f := clone(articleForm)
		f.Set("title", title)
		f.Set("category_ids", "1")
		id := entryIDFromLocation(t, author.post("/admin/entries/new?type=article", f))
		author.post("/admin/entries/"+id+"/transition", url.Values{"name": {"submit"}})
		ids = append(ids, id)
	}
	// Автор не может опубликовать сам.
	if resp := author.post("/admin/entries/"+ids[0]+"/transition", url.Values{"name": {"publish"}}); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("автор опубликовал сам: %d", resp.StatusCode)
	}

	// 4. Редактор видит очередь модерации и публикует пакетно.
	ed := e.login("editor@v.ru")
	_, body := ed.get("/admin/entries?status=review")
	if !strings.Contains(body, "Первый") || !strings.Contains(body, "Второй") {
		t.Fatal("материалов нет в очереди модерации")
	}
	resp = ed.post("/admin/entries/batch", url.Values{"action": {"publish"}, "ids": ids, "back": {"/admin/entries?status=review"}})
	flash := resp.Cookies()
	if resp.StatusCode != http.StatusSeeOther || len(flash) == 0 || !strings.Contains(mustUnescape(flash[0].Value), "Выполнено: 2") {
		t.Fatalf("пакетная публикация: %d %v", resp.StatusCode, flash)
	}
	// Повторная публикация — оба пропущены как недопустимый переход.
	resp = ed.post("/admin/entries/batch", url.Values{"action": {"publish"}, "ids": ids})
	if msg := mustUnescape(resp.Cookies()[0].Value); !strings.Contains(msg, "Выполнено: 0") || !strings.Contains(msg, "недопустимый переход): 2") {
		t.Fatalf("повторная публикация: %q", msg)
	}

	// 5. Всё видно в журнале действий.
	_, body = adm.get("/admin/audit?action=entry.publish")
	if strings.Count(body, "entry.publish") < 3 { // два в таблице + опция фильтра
		t.Fatalf("публикации не попали в журнал")
	}
	_, body = adm.get("/admin/audit?action=user.")
	if !strings.Contains(body, "new@v.ru") {
		t.Fatal("создание пользователя не попало в журнал")
	}
}

func TestRevisionsAndRollbackViaUI(t *testing.T) {
	e := newEnv(t)
	c := e.login("editor@v.ru")
	id := entryIDFromLocation(t, c.post("/admin/entries/new?type=article", clone(articleForm)))
	changed := clone(articleForm)
	changed.Set("title", "Изменённый заголовок")
	c.post("/admin/entries/"+id, changed)
	_, body := c.get("/admin/entries/" + id + "/revisions")
	if !strings.Contains(body, "Ревизия 2") || !strings.Contains(body, "Ревизия 1") {
		t.Fatal("ревизии не показаны")
	}
	c.post("/admin/entries/"+id+"/rollback", url.Values{"number": {"1"}})
	_, body = c.get("/admin/entries/" + id)
	if !strings.Contains(body, `value="Мост откроют в ноябре"`) {
		t.Fatal("откат через интерфейс не восстановил заголовок")
	}
}

// Каждая страница панели рендерится без ошибок шаблона (ошибка шаблона = 500).
func TestAllPagesRender(t *testing.T) {
	e := newEnv(t)
	c := e.login("admin@v.ru")
	id := entryIDFromLocation(t, c.post("/admin/entries/new?type=article", clone(articleForm)))
	c.post("/admin/terms", url.Values{"vocabulary": {"category"}, "name": {"Город"}})
	pages := []string{"/admin/", "/admin/entries", "/admin/entries?status=review&q=мост&page=2",
		"/admin/entries/" + id, "/admin/entries/" + id + "/revisions", "/admin/terms", "/admin/terms?edit=1",
		"/admin/media", "/admin/users", "/admin/users?edit=1", "/admin/audit", "/admin/audit?action=entry.",
		"/admin/entries/999"}
	for _, typ := range e.app.Types.List() {
		pages = append(pages, "/admin/entries/new?type="+typ.Name)
	}
	for _, p := range pages {
		resp, body := c.get(p)
		if resp.StatusCode >= 500 || (resp.StatusCode != 200 && !strings.HasSuffix(p, "999")) {
			t.Errorf("%s: %d %s", p, resp.StatusCode, body[:min(len(body), 200)])
		}
	}
	if resp, _ := e.anon().get("/admin/login"); resp.StatusCode != 200 {
		t.Errorf("/admin/login: %d", resp.StatusCode)
	}
}

func mustUnescape(s string) string {
	out, _ := url.QueryUnescape(s)
	return out
}
