package admin

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/ykkssyaa/vestnik-cms/internal/access"
	"github.com/ykkssyaa/vestnik-cms/internal/content"
	"github.com/ykkssyaa/vestnik-cms/internal/store"
)

const perPage = 20

type entriesData struct {
	Entries    []*content.Entry
	Pager      Pager
	Categories []content.Term
	Authors    []*store.User
	Statuses   []statusOption
	Batch      []batchAction
	Filter     store.EntryFilter
	OnlyOwn    bool
}

type statusOption struct{ Value, Label string }

type batchAction struct{ Name, Label string }

var statusOptions = []statusOption{
	{content.StatusDraft, content.StatusLabels[content.StatusDraft]},
	{content.StatusReview, content.StatusLabels[content.StatusReview]},
	{content.StatusPublished, content.StatusLabels[content.StatusPublished]},
	{content.StatusArchived, content.StatusLabels[content.StatusArchived]},
}

var batchActions = []batchAction{
	{"submit", "Отправить на модерацию"},
	{"publish", "Опубликовать"},
	{"archive", "В архив"},
	{"delete", "Удалить"},
}

func (h *Handler) entries(w http.ResponseWriter, r *http.Request) {
	cur := current(r)
	q := r.URL.Query()
	page := pageNum(r)
	f := store.EntryFilter{
		Type: q.Get("type"), Status: q.Get("status"), AuthorID: atoi(q.Get("author")),
		CategoryID: atoi(q.Get("category")), Query: q.Get("q"),
		Limit: perPage, Offset: (page - 1) * perPage,
	}
	onlyOwn := !h.Policy.Can(cur.Actor(), "entry.list.any", access.Resource{})
	if onlyOwn {
		f.AuthorID = cur.User.ID // фильтр навязывается: автор видит только свои материалы
	}
	list, total, err := h.Content.List(r.Context(), f)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	cats, _ := h.Taxonomy.List(r.Context(), content.VocabCategory)
	var authors []*store.User
	if !onlyOwn {
		authors, _ = store.NewUserRepo(h.Content.DB).List(r.Context())
	}
	title := "Материалы"
	if f.Status == content.StatusReview {
		title = "Модерация"
	}
	h.render(w, r, http.StatusOK, "entries", h.page(w, r, "entries", title, entriesData{
		Entries: list, Pager: newPager(page, perPage, total), Categories: cats, Authors: authors,
		Statuses: statusOptions, Batch: batchActions, Filter: f, OnlyOwn: onlyOwn,
	}))
}

// batch применяет действие к выбранным материалам. Каждый проверяется ядром отдельно.
func (h *Handler) batch(w http.ResponseWriter, r *http.Request) {
	cur := current(r)
	if !h.Policy.Can(cur.Actor(), "entry.batch", access.Resource{}) {
		h.forbidden(w, r)
		return
	}
	action := r.FormValue("action")
	valid := false
	for _, a := range batchActions {
		valid = valid || a.Name == action
	}
	if !valid {
		h.redirect(w, r, "/admin/entries", "Неизвестное пакетное действие")
		return
	}
	done, skipped := 0, map[string]int{}
	for _, raw := range r.Form["ids"] {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			continue
		}
		if action == "delete" {
			err = h.Content.Delete(r.Context(), cur.Actor(), id)
		} else {
			_, err = h.Content.Transition(r.Context(), cur.Actor(), id, action)
		}
		switch {
		case err == nil:
			done++
		case errors.Is(err, content.ErrBadTransition):
			skipped["недопустимый переход"]++
		case errors.Is(err, content.ErrForbidden):
			skipped["нет прав"]++
		case errors.Is(err, content.ErrNotFound):
			skipped["не найден"]++
		default:
			h.Log.Error("batch", "id", id, "err", err)
			skipped["ошибка"]++
		}
	}
	msg := fmt.Sprintf("Выполнено: %d", done)
	for reason, n := range skipped {
		msg += fmt.Sprintf("; пропущено (%s): %d", reason, n)
	}
	back := r.FormValue("back")
	if !strings.HasPrefix(back, "/admin/entries") {
		back = "/admin/entries"
	}
	h.redirect(w, r, back, msg)
}

type formData struct {
	Type        *content.Type
	Entry       *content.Entry
	Input       content.Input
	Errors      content.FieldErrors
	Categories  []content.Term
	Media       []*store.Media
	Transitions []content.Transition
	ReadOnly    bool
	TagsText    string
	Selected    map[int64]bool
}

// Value — значение поля для формы в виде строки.
func (f formData) Value(name string) string {
	v, ok := f.Input.Data[name]
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

// Checked — отмечен ли checkbox поля.
func (f formData) Checked(name string) bool {
	switch v := f.Input.Data[name].(type) {
	case bool:
		return v
	case string:
		return v == "on" || v == "true" || v == "1"
	}
	return false
}

func (f formData) Err(name string) string { return f.Errors[name] }

func (h *Handler) formFor(r *http.Request, typ *content.Type) formData {
	cats, _ := h.Taxonomy.List(r.Context(), content.VocabCategory)
	mediaList, _, _ := h.Media.List(r.Context(), 200, 0)
	return formData{Type: typ, Categories: cats, Media: mediaList, Selected: map[int64]bool{}}
}

// inputFromForm собирает ввод из формы по схеме типа.
func inputFromForm(r *http.Request, typ *content.Type) content.Input {
	in := content.Input{
		Title: r.FormValue("title"), Slug: r.FormValue("slug"),
		SEOTitle: r.FormValue("seo_title"), SEODescription: r.FormValue("seo_description"),
		Data: map[string]any{},
	}
	for _, f := range typ.Fields {
		if v, ok := r.Form["data."+f.Name]; ok && len(v) > 0 {
			in.Data[f.Name] = v[0]
		}
	}
	for _, raw := range r.Form["category_ids"] {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
			in.CategoryIDs = append(in.CategoryIDs, id)
		}
	}
	for _, t := range strings.Split(r.FormValue("tags"), ",") {
		if t = strings.TrimSpace(t); t != "" {
			in.Tags = append(in.Tags, t)
		}
	}
	return in
}

func (h *Handler) newEntry(w http.ResponseWriter, r *http.Request) {
	typ, ok := h.Content.Types.Get(r.URL.Query().Get("type"))
	if !ok {
		h.fail(w, r, store.ErrNotFound)
		return
	}
	if !h.Policy.Can(current(r).Actor(), "entry.create", access.Resource{Type: typ.Name}) {
		h.forbidden(w, r)
		return
	}
	fd := h.formFor(r, typ)
	fd.Input.Data = map[string]any{}
	for _, f := range typ.Fields {
		if f.Default != nil {
			fd.Input.Data[f.Name] = f.Default
		}
	}
	h.render(w, r, http.StatusOK, "entry_form", h.page(w, r, "entries", "Новый материал: "+typ.Label, fd))
}

func (h *Handler) createEntry(w http.ResponseWriter, r *http.Request) {
	typ, ok := h.Content.Types.Get(r.URL.Query().Get("type"))
	if !ok {
		h.fail(w, r, store.ErrNotFound)
		return
	}
	in := inputFromForm(r, typ)
	e, err := h.Content.Create(r.Context(), current(r).Actor(), typ.Name, in)
	var fe content.FieldErrors
	if errors.As(err, &fe) {
		fd := h.formFor(r, typ)
		fd.Input, fd.Errors, fd.TagsText = in, fe, r.FormValue("tags")
		for _, id := range in.CategoryIDs {
			fd.Selected[id] = true
		}
		h.render(w, r, http.StatusUnprocessableEntity, "entry_form", h.page(w, r, "entries", "Новый материал: "+typ.Label, fd))
		return
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.redirect(w, r, fmt.Sprintf("/admin/entries/%d", e.ID), "Черновик создан")
}

// loadVisible загружает материал и проверяет, что пользователь вправе его видеть.
func (h *Handler) loadVisible(w http.ResponseWriter, r *http.Request) (*content.Entry, *content.Type, bool) {
	e, err := h.Content.Get(r.Context(), pathID(r))
	if err != nil {
		h.fail(w, r, err)
		return nil, nil, false
	}
	cur := current(r)
	if e.AuthorID != cur.User.ID && !h.Policy.Can(cur.Actor(), "entry.list.any", access.Resource{}) {
		h.forbidden(w, r)
		return nil, nil, false
	}
	typ, ok := h.Content.Types.Get(e.Type)
	if !ok {
		h.fail(w, r, content.ErrUnknownType)
		return nil, nil, false
	}
	return e, typ, true
}

func (h *Handler) editForm(r *http.Request, e *content.Entry, typ *content.Type) formData {
	cur := current(r)
	fd := h.formFor(r, typ)
	fd.Entry = e
	fd.Input = content.Input{Title: e.Title, Slug: e.Slug, SEOTitle: e.SEOTitle, SEODescription: e.SEODescription, Data: e.Data}
	var tags []string
	for _, t := range e.Tags {
		tags = append(tags, t.Name)
	}
	fd.TagsText = strings.Join(tags, ", ")
	for _, c := range e.Categories {
		fd.Selected[c.ID] = true
	}
	fd.Transitions = h.Content.Available(cur.Actor(), e)
	fd.ReadOnly = !h.Policy.Can(cur.Actor(), "entry.edit", access.Resource{Type: e.Type, AuthorID: e.AuthorID, Status: e.Status})
	return fd
}

func (h *Handler) editEntry(w http.ResponseWriter, r *http.Request) {
	e, typ, ok := h.loadVisible(w, r)
	if !ok {
		return
	}
	h.render(w, r, http.StatusOK, "entry_form", h.page(w, r, "entries", e.Title, h.editForm(r, e, typ)))
}

func (h *Handler) updateEntry(w http.ResponseWriter, r *http.Request) {
	e, typ, ok := h.loadVisible(w, r)
	if !ok {
		return
	}
	in := inputFromForm(r, typ)
	_, err := h.Content.Update(r.Context(), current(r).Actor(), e.ID, in)
	var fe content.FieldErrors
	if errors.As(err, &fe) {
		fd := h.editForm(r, e, typ)
		fd.Input, fd.Errors, fd.TagsText = in, fe, r.FormValue("tags")
		fd.Selected = map[int64]bool{}
		for _, id := range in.CategoryIDs {
			fd.Selected[id] = true
		}
		h.render(w, r, http.StatusUnprocessableEntity, "entry_form", h.page(w, r, "entries", e.Title, fd))
		return
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.redirect(w, r, fmt.Sprintf("/admin/entries/%d", e.ID), "Сохранено")
}

func (h *Handler) transition(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	e, err := h.Content.Transition(r.Context(), current(r).Actor(), id, r.FormValue("name"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.redirect(w, r, fmt.Sprintf("/admin/entries/%d", id), "Статус: "+content.StatusLabels[e.Status])
}

func (h *Handler) deleteEntry(w http.ResponseWriter, r *http.Request) {
	if err := h.Content.Delete(r.Context(), current(r).Actor(), pathID(r)); err != nil {
		h.fail(w, r, err)
		return
	}
	h.redirect(w, r, "/admin/entries", "Материал удалён")
}

type revisionView struct {
	*content.Revision
	Snap content.Snapshot
}

type revisionsData struct {
	Entry       *content.Entry
	Type        *content.Type
	Revisions   []revisionView
	CanRollback bool
}

func (h *Handler) revisions(w http.ResponseWriter, r *http.Request) {
	e, typ, ok := h.loadVisible(w, r)
	if !ok {
		return
	}
	revs, err := h.Content.Revisions(r.Context(), e.ID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	views := make([]revisionView, 0, len(revs))
	for _, rev := range revs {
		snap, _ := content.ParseSnapshot(rev)
		snap.Data = typ.Normalize(snap.Data)
		views = append(views, revisionView{Revision: rev, Snap: snap})
	}
	cur := current(r)
	h.render(w, r, http.StatusOK, "revisions", h.page(w, r, "entries", "Ревизии: "+e.Title, revisionsData{
		Entry: e, Type: typ, Revisions: views,
		CanRollback: h.Policy.Can(cur.Actor(), "entry.rollback", access.Resource{Type: e.Type, AuthorID: e.AuthorID, Status: e.Status}),
	}))
}

func (h *Handler) rollback(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	n, _ := strconv.Atoi(r.FormValue("number"))
	if _, err := h.Content.Rollback(r.Context(), current(r).Actor(), id, n); err != nil {
		var fe content.FieldErrors
		if errors.As(err, &fe) {
			h.redirect(w, r, fmt.Sprintf("/admin/entries/%d/revisions", id), "Ревизия не проходит текущую схему типа: "+fe.Error())
			return
		}
		h.fail(w, r, err)
		return
	}
	h.redirect(w, r, fmt.Sprintf("/admin/entries/%d", id), fmt.Sprintf("Восстановлена ревизия %d", n))
}
