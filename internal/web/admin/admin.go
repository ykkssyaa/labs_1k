// Package admin — панель администрирования: серверные HTML-формы.
// Права проверяются через access.Policy, бизнес-правила — в ядре (content).
package admin

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/ykkssyaa/vestnik-cms/internal/access"
	"github.com/ykkssyaa/vestnik-cms/internal/auth"
	"github.com/ykkssyaa/vestnik-cms/internal/content"
	"github.com/ykkssyaa/vestnik-cms/internal/media"
	"github.com/ykkssyaa/vestnik-cms/internal/render"
	"github.com/ykkssyaa/vestnik-cms/internal/store"
)

const (
	sessionCookie = "vestnik_session"
	flashCookie   = "vestnik_flash"
	theme         = "_admin"
)

type Handler struct {
	Content  *content.Service
	Taxonomy *content.Taxonomy
	Media    *media.Service
	Auth     *auth.Service
	Policy   *access.Policy
	Render   *render.Engine
	Settings *store.SettingsRepo
	// ActiveTheme — текущая тема сайта (решает публичная часть).
	ActiveTheme func(ctx context.Context) string
	Log         *slog.Logger
	// SecureCookie — ставить Secure на cookie (за HTTPS).
	SecureCookie bool
}

type ctxKey struct{}

func current(r *http.Request) *auth.Current {
	cur, _ := r.Context().Value(ctxKey{}).(*auth.Current)
	return cur
}

// Page — общие данные каждой страницы панели.
type Page struct {
	Title   string
	Section string
	Cur     *auth.Current
	Flash   string
	Error   string
	Types   []*content.Type
	Query   url.Values
	Data    any
	policy  *access.Policy
}

// Can — проверка права из шаблона: {{if .Can "taxonomy.manage"}}.
func (p *Page) Can(action string) bool {
	return p.Cur != nil && p.policy.Can(p.Cur.Actor(), action, access.Resource{})
}

// CanType — может ли пользователь создавать материалы типа.
func (p *Page) CanType(action, typ string) bool {
	return p.Cur != nil && p.policy.Can(p.Cur.Actor(), action, access.Resource{Type: typ})
}

// CreatableTypes — типы, которые пользователь может создавать.
func (p *Page) CreatableTypes() []*content.Type {
	var out []*content.Type
	for _, t := range p.Types {
		if p.CanType("entry.create", t.Name) {
			out = append(out, t)
		}
	}
	return out
}

func (h *Handler) page(w http.ResponseWriter, r *http.Request, section, title string, data any) *Page {
	p := &Page{Title: title, Section: section, Cur: current(r), Types: h.Content.Types.List(),
		Query: r.URL.Query(), Data: data, policy: h.Policy}
	if c, err := r.Cookie(flashCookie); err == nil {
		p.Flash, _ = url.QueryUnescape(c.Value)
		http.SetCookie(w, &http.Cookie{Name: flashCookie, Path: "/admin", MaxAge: -1})
	}
	return p
}

func (h *Handler) render(w http.ResponseWriter, r *http.Request, status int, name string, p *Page) {
	// Сначала рендерим в буфер: при ошибке шаблона клиент получит 500, а не 200 с обрезанной страницей.
	var buf bytes.Buffer
	if err := h.Render.Render(&buf, theme, name, p); err != nil {
		h.Log.Error("render admin", "page", name, "err", err)
		http.Error(w, "Ошибка шаблона", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	buf.WriteTo(w)
}

// redirect с сообщением, которое покажется на следующей странице.
func (h *Handler) redirect(w http.ResponseWriter, r *http.Request, to, flash string) {
	if flash != "" {
		http.SetCookie(w, &http.Cookie{Name: flashCookie, Value: url.QueryEscape(flash), Path: "/admin",
			HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 60})
	}
	http.Redirect(w, r, to, http.StatusSeeOther)
}

// fail переводит ошибку ядра в HTTP-ответ.
func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	msg := "Внутренняя ошибка"
	switch {
	case errors.Is(err, store.ErrNotFound):
		status, msg = http.StatusNotFound, "Не найдено"
	case errors.Is(err, content.ErrForbidden), errors.Is(err, media.ErrForbidden), errors.Is(err, auth.ErrForbidden):
		status, msg = http.StatusForbidden, "Недостаточно прав"
	case errors.Is(err, content.ErrBadTransition):
		status, msg = http.StatusConflict, err.Error()
	default:
		h.Log.Error("admin", "path", r.URL.Path, "err", err)
	}
	p := h.page(w, r, "", msg, nil)
	p.Error = msg
	h.render(w, r, status, "error", p)
}

func (h *Handler) forbidden(w http.ResponseWriter, r *http.Request) {
	h.fail(w, r, content.ErrForbidden)
}

// Routes регистрирует маршруты панели.
func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/login", h.loginForm)
	mux.HandleFunc("POST /admin/login", h.login)

	in := func(pattern string, fn http.HandlerFunc) { mux.Handle(pattern, h.requireLogin(fn)) }
	in("POST /admin/logout", h.logout)
	in("GET /admin/{$}", h.dashboard)
	in("GET /admin/entries", h.entries)
	in("POST /admin/entries/batch", h.batch)
	in("GET /admin/entries/new", h.newEntry)
	in("POST /admin/entries/new", h.createEntry)
	in("GET /admin/entries/{id}", h.editEntry)
	in("POST /admin/entries/{id}", h.updateEntry)
	in("POST /admin/entries/{id}/transition", h.transition)
	in("POST /admin/entries/{id}/delete", h.deleteEntry)
	in("GET /admin/entries/{id}/revisions", h.revisions)
	in("POST /admin/entries/{id}/rollback", h.rollback)
	in("GET /admin/terms", h.terms)
	in("POST /admin/terms", h.saveTerm)
	in("POST /admin/terms/{id}/delete", h.deleteTerm)
	in("GET /admin/media", h.mediaList)
	in("POST /admin/media", h.upload)
	in("POST /admin/media/{id}/alt", h.mediaAlt)
	in("POST /admin/media/{id}/delete", h.mediaDelete)
	in("GET /admin/users", h.users)
	in("POST /admin/users", h.saveUser)
	in("GET /admin/audit", h.audit)
	in("GET /admin/settings", h.settings)
	in("POST /admin/settings", h.saveSettings)
	mux.Handle("GET /admin", http.RedirectHandler("/admin/", http.StatusMovedPermanently))
}

// requireLogin пускает только вошедших с правом admin.access и проверяет CSRF у POST.
func (h *Handler) requireLogin(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var token string
		if c, err := r.Cookie(sessionCookie); err == nil {
			token = c.Value
		}
		cur, err := h.Auth.Lookup(r.Context(), token)
		if err != nil {
			http.Redirect(w, r, "/admin/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), ctxKey{}, cur))
		if !h.Policy.Can(cur.Actor(), "admin.access", access.Resource{}) {
			h.forbidden(w, r)
			return
		}
		if r.Method == http.MethodPost {
			// Для multipart (загрузка медиа) форму разбирает ParseMultipartForm, для остальных — ParseForm.
			r.ParseMultipartForm(media.MaxSize + 1<<20)
			if !auth.CheckCSRF(cur, r.FormValue("csrf")) {
				http.Error(w, "CSRF-токен не совпадает. Обновите страницу и повторите.", http.StatusForbidden)
				return
			}
		}
		next(w, r)
	})
}

func (h *Handler) loginForm(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusOK, "login", h.page(w, r, "", "Вход", map[string]string{"Next": r.URL.Query().Get("next")}))
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	token, _, err := h.Auth.Login(r.Context(), r.FormValue("email"), r.FormValue("password"))
	if err != nil {
		p := h.page(w, r, "", "Вход", map[string]string{"Next": r.FormValue("next"), "Email": r.FormValue("email")})
		p.Error = "Неверный email или пароль"
		h.render(w, r, http.StatusUnauthorized, "login", p)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: h.SecureCookie, Expires: time.Now().Add(auth.SessionTTL)})
	next := r.FormValue("next")
	// Только локальные пути: иначе ?next= превращается в открытый редирект.
	if u, err := url.Parse(next); err != nil || next == "" || u.IsAbs() || u.Host != "" || len(next) < 2 || next[0] != '/' || next[1] == '/' || next[1] == '\\' {
		next = "/admin/"
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		h.Auth.Logout(r.Context(), c.Value, current(r))
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

func pathID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id
}

func atoi(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

func pageNum(r *http.Request) int {
	n, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if n < 1 {
		return 1
	}
	return n
}

// Pager — данные для partial-шаблона пагинации.
type Pager struct {
	Page, Pages, Total int
}

func newPager(page, perPage, total int) Pager {
	pages := (total + perPage - 1) / perPage
	if pages < 1 {
		pages = 1
	}
	return Pager{Page: page, Pages: pages, Total: total}
}
