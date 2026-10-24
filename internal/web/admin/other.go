package admin

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/ykkssyaa/vestnik-cms/internal/access"
	"github.com/ykkssyaa/vestnik-cms/internal/auth"
	"github.com/ykkssyaa/vestnik-cms/internal/content"
	"github.com/ykkssyaa/vestnik-cms/internal/media"
	"github.com/ykkssyaa/vestnik-cms/internal/store"
)

type dashboardData struct {
	Statuses []statusOption
	Counts   map[string]int
	Review   []*content.Entry
	MyDrafts []*content.Entry
}

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	cur := current(r)
	ctx := r.Context()
	d := dashboardData{Statuses: statusOptions, Counts: map[string]int{}}
	base := store.EntryFilter{Limit: 1}
	if !h.Policy.Can(cur.Actor(), "entry.list.any", access.Resource{}) {
		base.AuthorID = cur.User.ID
	}
	for _, st := range statusOptions {
		f := base
		f.Status = st.Value
		_, d.Counts[st.Value], _ = h.Content.List(ctx, f)
	}
	if h.Policy.Can(cur.Actor(), "entry.transition.publish", access.Resource{}) {
		d.Review, _, _ = h.Content.List(ctx, store.EntryFilter{Status: content.StatusReview, Limit: 10})
	}
	d.MyDrafts, _, _ = h.Content.List(ctx, store.EntryFilter{Status: content.StatusDraft, AuthorID: cur.User.ID, Limit: 10})
	h.render(w, r, http.StatusOK, "dashboard", h.page(w, r, "dashboard", "Сводка", d))
}

type termsData struct {
	Categories []content.Term
	Tags       []content.Term
	Errors     content.FieldErrors
	Edit       *content.Term
}

func (h *Handler) termsPage(w http.ResponseWriter, r *http.Request, status int, d termsData) {
	d.Categories, _ = h.Taxonomy.List(r.Context(), content.VocabCategory)
	d.Tags, _ = h.Taxonomy.List(r.Context(), content.VocabTag)
	h.render(w, r, status, "terms", h.page(w, r, "terms", "Рубрики и теги", d))
}

func (h *Handler) terms(w http.ResponseWriter, r *http.Request) {
	if !h.Policy.Can(current(r).Actor(), "taxonomy.manage", access.Resource{}) {
		h.forbidden(w, r)
		return
	}
	d := termsData{}
	if id := atoi(r.URL.Query().Get("edit")); id != 0 {
		d.Edit, _ = store.NewTermRepo(h.Taxonomy.DB).Get(r.Context(), id)
	}
	h.termsPage(w, r, http.StatusOK, d)
}

// saveTerm создаёт термин или (если передан id) изменяет существующий.
func (h *Handler) saveTerm(w http.ResponseWriter, r *http.Request) {
	term := &content.Term{
		ID: atoi(r.FormValue("id")), Vocabulary: r.FormValue("vocabulary"),
		Name: r.FormValue("name"), Slug: strings.TrimSpace(r.FormValue("slug")), ParentID: atoi(r.FormValue("parent_id")),
	}
	var err error
	if term.ID != 0 {
		err = h.Taxonomy.Update(r.Context(), current(r).Actor(), term)
	} else {
		err = h.Taxonomy.Create(r.Context(), current(r).Actor(), term)
	}
	var fe content.FieldErrors
	switch {
	case errors.As(err, &fe):
		h.termsPage(w, r, http.StatusUnprocessableEntity, termsData{Errors: fe, Edit: term})
	case errors.Is(err, content.ErrTermCycle):
		h.termsPage(w, r, http.StatusUnprocessableEntity, termsData{Errors: content.FieldErrors{"parent_id": err.Error()}, Edit: term})
	case err != nil && strings.Contains(err.Error(), "UNIQUE"):
		h.termsPage(w, r, http.StatusUnprocessableEntity, termsData{Errors: content.FieldErrors{"slug": "такой адрес уже занят"}, Edit: term})
	case err != nil:
		h.fail(w, r, err)
	default:
		h.redirect(w, r, "/admin/terms", "Сохранено: "+term.Name)
	}
}

func (h *Handler) deleteTerm(w http.ResponseWriter, r *http.Request) {
	if err := h.Taxonomy.Delete(r.Context(), current(r).Actor(), pathID(r)); err != nil {
		h.fail(w, r, err)
		return
	}
	h.redirect(w, r, "/admin/terms", "Удалено")
}

type mediaData struct {
	Items []*store.Media
	Pager Pager
}

func (h *Handler) mediaList(w http.ResponseWriter, r *http.Request) {
	page := pageNum(r)
	items, total, err := h.Media.List(r.Context(), 24, (page-1)*24)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.render(w, r, http.StatusOK, "media", h.page(w, r, "media", "Медиатека", mediaData{Items: items, Pager: newPager(page, 24, total)}))
}

func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		h.redirect(w, r, "/admin/media", "Выберите файл")
		return
	}
	defer file.Close()
	m, err := h.Media.Upload(r.Context(), current(r).Actor(), header.Filename, file)
	switch {
	case errors.Is(err, media.ErrUnsupported), errors.Is(err, media.ErrTooLarge):
		h.redirect(w, r, "/admin/media", err.Error())
		return
	case err != nil:
		h.fail(w, r, err)
		return
	}
	if alt := r.FormValue("alt"); alt != "" {
		h.Media.SetAlt(r.Context(), current(r).Actor(), m.ID, alt)
	}
	h.Auth.Record(r.Context(), h.Content.DB, current(r).Actor(), "media.upload", "media", m.ID, m.Filename)
	h.redirect(w, r, "/admin/media", fmt.Sprintf("Загружено: %s (%d×%d)", m.Filename, m.Width, m.Height))
}

func (h *Handler) mediaAlt(w http.ResponseWriter, r *http.Request) {
	if err := h.Media.SetAlt(r.Context(), current(r).Actor(), pathID(r), r.FormValue("alt")); err != nil {
		h.fail(w, r, err)
		return
	}
	h.redirect(w, r, "/admin/media", "Описание сохранено")
}

func (h *Handler) mediaDelete(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if err := h.Media.Delete(r.Context(), current(r).Actor(), id); err != nil {
		h.fail(w, r, err)
		return
	}
	h.Auth.Record(r.Context(), h.Content.DB, current(r).Actor(), "media.delete", "media", id, "")
	h.redirect(w, r, "/admin/media", "Файл удалён")
}

type usersData struct {
	Users  []*store.User
	Roles  []*access.Role
	Errors auth.FormErrors
	Edit   *store.User
}

func (h *Handler) usersPage(w http.ResponseWriter, r *http.Request, status int, d usersData) {
	d.Users, _ = store.NewUserRepo(h.Content.DB).List(r.Context())
	d.Roles = h.Policy.Roles()
	h.render(w, r, status, "users", h.page(w, r, "users", "Пользователи", d))
}

func (h *Handler) users(w http.ResponseWriter, r *http.Request) {
	if !h.Policy.Can(current(r).Actor(), "users.manage", access.Resource{}) {
		h.forbidden(w, r)
		return
	}
	d := usersData{}
	if id := atoi(r.URL.Query().Get("edit")); id != 0 {
		d.Edit, _ = store.NewUserRepo(h.Content.DB).Get(r.Context(), id)
	}
	h.usersPage(w, r, http.StatusOK, d)
}

func (h *Handler) saveUser(w http.ResponseWriter, r *http.Request) {
	in := auth.UserInput{Email: r.FormValue("email"), Name: r.FormValue("name"), Role: r.FormValue("role"),
		Password: r.FormValue("password"), Active: r.FormValue("active") == "on"}
	id := atoi(r.FormValue("id"))
	var err error
	if id != 0 {
		err = h.Auth.UpdateUser(r.Context(), current(r).Actor(), id, in)
	} else {
		_, err = h.Auth.CreateUser(r.Context(), current(r).Actor(), in)
	}
	var fe auth.FormErrors
	if errors.As(err, &fe) {
		h.usersPage(w, r, http.StatusUnprocessableEntity, usersData{Errors: fe,
			Edit: &store.User{ID: id, Email: in.Email, Name: in.Name, Role: in.Role, Active: in.Active}})
		return
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.redirect(w, r, "/admin/users", "Пользователь сохранён: "+in.Email)
}

type auditData struct {
	Records []store.AuditRecord
	Users   []*store.User
	Pager   Pager
	Actions []string
}

var auditActions = []string{"entry.", "entry.create", "entry.update", "entry.publish", "entry.delete",
	"entry.rollback", "term.", "media.", "user.", "auth."}

func (h *Handler) audit(w http.ResponseWriter, r *http.Request) {
	if !h.Policy.Can(current(r).Actor(), "audit.view", access.Resource{}) {
		h.forbidden(w, r)
		return
	}
	page := pageNum(r)
	q := r.URL.Query()
	recs, total, err := store.NewAuditRepo(h.Content.DB).List(r.Context(), store.AuditFilter{
		UserID: atoi(q.Get("user")), Action: q.Get("action"), Limit: 50, Offset: (page - 1) * 50,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	users, _ := store.NewUserRepo(h.Content.DB).List(r.Context())
	h.render(w, r, http.StatusOK, "audit", h.page(w, r, "audit", "Журнал действий", auditData{
		Records: recs, Users: users, Pager: newPager(page, 50, total), Actions: auditActions,
	}))
}
