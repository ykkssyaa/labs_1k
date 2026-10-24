// Package testkit — общая обвязка HTTP-тестов: поднятое приложение и «браузер» пользователя.
// Используется только из тестов.
package testkit

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

const Password = "password1"

type Env struct {
	T   *testing.T
	App *app.App
	Srv *httptest.Server
}

// NewEnv поднимает приложение на БД в памяти с пользователями admin@, editor@, author@, author2@v.ru.
// root — путь к корню репозитория относительно пакета теста.
func NewEnv(t *testing.T, root string) *Env {
	t.Helper()
	cfg := app.TestConfig(root)
	cfg.UploadsDir = t.TempDir()
	a, err := app.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(a.Handler())
	t.Cleanup(func() { srv.Close(); a.Close() })
	e := &Env{T: t, App: a, Srv: srv}
	e.User("admin@v.ru", "Админ", "admin")
	e.User("editor@v.ru", "Редактор", "editor")
	e.User("author@v.ru", "Автор", "author")
	e.User("author2@v.ru", "Другой автор", "author")
	return e
}

func (e *Env) User(email, name, role string) *store.User {
	u, err := e.App.Auth.CreateUser(context.Background(), access.Actor{},
		auth.UserInput{Email: email, Name: name, Role: role, Password: Password})
	if err != nil {
		e.T.Fatal(err)
	}
	return u
}

// Actor — пользователь как субъект ядра (для подготовки данных в обход HTTP).
func (e *Env) Actor(email string) access.Actor {
	u, err := store.NewUserRepo(e.App.DB).ByEmail(context.Background(), email)
	if err != nil {
		e.T.Fatal(err)
	}
	return access.Actor{ID: u.ID, Name: u.Name, Role: u.Role}
}

// Client — браузер одного пользователя: cookie и CSRF-токен последней страницы.
type Client struct {
	e    *Env
	HTTP *http.Client
	CSRF string
}

func (e *Env) Anon() *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{e: e, HTTP: &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse // редиректы проверяют сами тесты
	}}}
}

func (e *Env) Login(email string) *Client {
	c := e.Anon()
	resp := c.Post("/admin/login", url.Values{"email": {email}, "password": {Password}})
	if resp.StatusCode != http.StatusSeeOther {
		e.T.Fatalf("вход %s: %d", email, resp.StatusCode)
	}
	c.Get("/admin/") // получить CSRF-токен со страницы
	return c
}

var csrfRe = regexp.MustCompile(`name="csrf" value="([^"]+)"`)

func (c *Client) Do(req *http.Request) (*http.Response, string) {
	c.e.T.Helper()
	resp, err := c.HTTP.Do(req)
	if err != nil {
		c.e.T.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if m := csrfRe.FindSubmatch(body); m != nil {
		c.CSRF = string(m[1])
	}
	return resp, string(body)
}

func (c *Client) Get(path string) (*http.Response, string) {
	c.e.T.Helper()
	req, _ := http.NewRequest(http.MethodGet, c.e.Srv.URL+path, nil)
	return c.Do(req)
}

func (c *Client) Post(path string, form url.Values) *http.Response {
	c.e.T.Helper()
	resp, _ := c.PostBody(path, form)
	return resp
}

func (c *Client) PostBody(path string, form url.Values) (*http.Response, string) {
	c.e.T.Helper()
	if form.Get("csrf") == "" && c.CSRF != "" {
		form.Set("csrf", c.CSRF)
	}
	req, _ := http.NewRequest(http.MethodPost, c.e.Srv.URL+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.Do(req)
}

// EntryID достаёт id материала из редиректа после создания.
func EntryID(t *testing.T, resp *http.Response) string {
	t.Helper()
	m := regexp.MustCompile(`^/admin/entries/(\d+)$`).FindStringSubmatch(resp.Header.Get("Location"))
	if m == nil {
		t.Fatalf("ожидали редирект на материал, получили %d %q", resp.StatusCode, resp.Header.Get("Location"))
	}
	return m[1]
}

// Clone копирует значения формы, чтобы тесты могли менять их независимо.
func Clone(v url.Values) url.Values {
	out := url.Values{}
	for k, vs := range v {
		out[k] = append([]string(nil), vs...)
	}
	return out
}

// Flash — сообщение из cookie, которое панель показывает после редиректа.
func Flash(resp *http.Response) string {
	for _, c := range resp.Cookies() {
		if c.Name == "vestnik_flash" {
			s, _ := url.QueryUnescape(c.Value)
			return s
		}
	}
	return ""
}
