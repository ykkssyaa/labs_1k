// Package site — публичная часть: страницы в активной теме, sitemap, robots.
package site

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ykkssyaa/vestnik-cms/internal/content"
	"github.com/ykkssyaa/vestnik-cms/internal/render"
	"github.com/ykkssyaa/vestnik-cms/internal/store"
	"github.com/ykkssyaa/vestnik-cms/internal/web"
)

const perPage = 12

// ViewHook вызывается при показе материала (аналитика подключается на этапе 5).
type ViewHook func(r *http.Request, e *content.Entry)

type Handler struct {
	Content      *content.Service
	Taxonomy     *content.Taxonomy
	Render       *render.Engine
	Settings     *store.SettingsRepo
	Media        web.MediaLookup
	BaseURL      string
	DefaultTheme string
	SiteName     string
	OnView       ViewHook
	Log          *slog.Logger
}

type MenuItem struct {
	Title  string
	URL    string
	Active bool
}

// Meta — то, что попадает в <head>: заголовок, описание, canonical, OpenGraph.
type Meta struct {
	Title       string
	Description string
	Canonical   string
	Image       string
	OGType      string
}

// View — данные любой публичной страницы.
type View struct {
	SiteName string
	BaseURL  string
	Theme    string
	Menu     []MenuItem
	Meta     Meta
	Query    url.Values
	Data     any
}

func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", h.home)
	mux.HandleFunc("GET /{first}", h.typeOrPage)
	mux.HandleFunc("GET /{prefix}/{slug}", h.entry)
	mux.HandleFunc("GET /section/{slug}", h.section)
	mux.HandleFunc("GET /tag/{slug}", h.tag)
	mux.HandleFunc("GET /search", h.search)
	mux.HandleFunc("GET /sitemap.xml", h.sitemap)
	mux.HandleFunc("GET /robots.txt", h.robots)
}

// Theme — активная тема сайта из настроек.
func (h *Handler) Theme(ctx context.Context) string {
	name, err := h.Settings.Get(ctx, "theme", h.DefaultTheme)
	if err != nil || !h.Render.HasTheme(name) {
		return h.DefaultTheme
	}
	return name
}

func (h *Handler) view(r *http.Request, meta Meta, data any) *View {
	if meta.Canonical == "" {
		meta.Canonical = h.BaseURL + r.URL.Path
	}
	if meta.OGType == "" {
		meta.OGType = "website"
	}
	if meta.Title == "" {
		meta.Title = h.SiteName
	} else {
		meta.Title += " — " + h.SiteName
	}
	return &View{SiteName: h.SiteName, BaseURL: h.BaseURL, Theme: h.Theme(r.Context()), Menu: h.menu(r),
		Meta: meta, Query: r.URL.Query(), Data: data}
}

func (h *Handler) render(w http.ResponseWriter, r *http.Request, status int, page string, v *View) {
	var buf bytes.Buffer
	if err := h.Render.Render(&buf, v.Theme, page, v); err != nil {
		h.Log.Error("render site", "page", page, "theme", v.Theme, "err", err)
		http.Error(w, "Ошибка шаблона", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	buf.WriteTo(w)
}

func (h *Handler) notFound(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusNotFound, "404", h.view(r, Meta{Title: "Страница не найдена"}, nil))
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, store.ErrNotFound) {
		h.notFound(w, r)
		return
	}
	h.Log.Error("site", "path", r.URL.Path, "err", err)
	http.Error(w, "Внутренняя ошибка", http.StatusInternalServerError)
}

// menu — рубрики верхнего уровня и страницы с show_in_menu, по menu_order.
func (h *Handler) menu(r *http.Request) []MenuItem {
	ctx := r.Context()
	var items []MenuItem
	cats, _ := h.Taxonomy.List(ctx, content.VocabCategory)
	for _, c := range cats {
		if c.ParentID == 0 {
			items = append(items, MenuItem{Title: c.Name, URL: "/section/" + c.Slug})
		}
	}
	if pageType, ok := h.Content.Types.ByPrefix(""); ok {
		pages, _, _ := h.Content.List(ctx, store.EntryFilter{Type: pageType.Name, Status: content.StatusPublished, Limit: 50})
		sort.SliceStable(pages, func(i, j int) bool {
			return web.MediaID(pages[i].Data["menu_order"]) < web.MediaID(pages[j].Data["menu_order"])
		})
		for _, p := range pages {
			if show, _ := p.Data["show_in_menu"].(bool); show {
				items = append(items, MenuItem{Title: p.Title, URL: pageType.Path(p.Slug)})
			}
		}
	}
	for i := range items {
		items[i].Active = r.URL.Path == items[i].URL || strings.HasPrefix(r.URL.Path, items[i].URL+"/")
	}
	return items
}

// listTypes — типы, у которых есть лента (непустой префикс адреса).
func (h *Handler) listTypes() []*content.Type {
	var out []*content.Type
	for _, t := range h.Content.Types.List() {
		if t.URLPrefix != "" {
			out = append(out, t)
		}
	}
	return out
}

func (h *Handler) listTypeNames() []string {
	var out []string
	for _, t := range h.listTypes() {
		out = append(out, t.Name)
	}
	return out
}

func page(r *http.Request) int {
	n, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if n < 1 {
		return 1
	}
	return n
}

// ListData — лента материалов с пагинацией.
type ListData struct {
	Heading string
	Lead    string
	Entries []*content.Entry
	Page    int
	Pages   int
	Total   int
}

func (h *Handler) list(r *http.Request, f store.EntryFilter, heading string) (ListData, error) {
	p := page(r)
	f.Status, f.Sort, f.Limit, f.Offset = content.StatusPublished, "published", perPage, (p-1)*perPage
	entries, total, err := h.Content.List(r.Context(), f)
	pages := (total + perPage - 1) / perPage
	return ListData{Heading: heading, Entries: entries, Page: p, Pages: max(pages, 1), Total: total}, err
}

type homeBlock struct {
	Type    *content.Type
	Entries []*content.Entry
}

type HomeData struct {
	Latest []*content.Entry
	Blocks []homeBlock
}

func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	latest, _, err := h.Content.List(ctx, store.EntryFilter{Types: h.listTypeNames(), Status: content.StatusPublished,
		Sort: "published", Limit: 7})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var blocks []homeBlock
	for _, t := range h.listTypes() {
		list, _, _ := h.Content.List(ctx, store.EntryFilter{Type: t.Name, Status: content.StatusPublished, Sort: "published", Limit: 4})
		if len(list) > 0 {
			blocks = append(blocks, homeBlock{Type: t, Entries: list})
		}
	}
	h.render(w, r, http.StatusOK, "home", h.view(r, Meta{Description: "Городское онлайн-издание: новости, статьи, рецензии и афиша"},
		HomeData{Latest: latest, Blocks: blocks}))
}

// typeOrPage: /{first} — либо лента типа с таким префиксом, либо статическая страница.
func (h *Handler) typeOrPage(w http.ResponseWriter, r *http.Request) {
	first := r.PathValue("first")
	if t, ok := h.Content.Types.ByPrefix(first); ok && first != "" {
		data, err := h.list(r, store.EntryFilter{Type: t.Name}, t.LabelPlural)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		h.render(w, r, http.StatusOK, "list", h.view(r, Meta{Title: t.LabelPlural}, data))
		return
	}
	pageType, ok := h.Content.Types.ByPrefix("")
	if !ok {
		h.notFound(w, r)
		return
	}
	h.showEntry(w, r, pageType, first)
}

func (h *Handler) entry(w http.ResponseWriter, r *http.Request) {
	t, ok := h.Content.Types.ByPrefix(r.PathValue("prefix"))
	if !ok || t.URLPrefix == "" {
		h.notFound(w, r)
		return
	}
	h.showEntry(w, r, t, r.PathValue("slug"))
}

// EntryData — страница материала.
type EntryData struct {
	Entry   *content.Entry
	Type    *content.Type
	Related []*content.Entry
}

func (h *Handler) showEntry(w http.ResponseWriter, r *http.Request, t *content.Type, slug string) {
	e, err := h.Content.GetBySlug(r.Context(), t.Name, slug)
	if err != nil || e.Status != content.StatusPublished {
		h.notFound(w, r)
		return
	}
	if h.OnView != nil {
		h.OnView(r, e)
	}
	meta := Meta{Title: e.Title, Description: web.Summary(h.Content.Types, e, 160), OGType: "article",
		Canonical: h.BaseURL + t.Path(e.Slug)}
	if e.SEOTitle != "" {
		meta.Title = e.SEOTitle
	}
	for _, f := range t.Fields {
		if f.Type == content.FieldMedia {
			if m := h.Media(r.Context(), web.MediaID(e.Data[f.Name])); m != nil {
				meta.Image = h.BaseURL + web.MediaURL(m, "large")
				break
			}
		}
	}
	var related []*content.Entry
	if len(e.Categories) > 0 {
		list, _, _ := h.Content.List(r.Context(), store.EntryFilter{CategoryID: e.Categories[0].ID,
			Status: content.StatusPublished, Sort: "published", Limit: 4})
		for _, x := range list {
			if x.ID != e.ID && len(related) < 3 {
				related = append(related, x)
			}
		}
	}
	// Тема может дать отдельный шаблон для типа; иначе общий шаблон выводит поля по схеме.
	name := "entry-" + t.Name
	if !h.Render.Has(h.Theme(r.Context()), name) {
		name = "entry"
	}
	h.render(w, r, http.StatusOK, name, h.view(r, meta, EntryData{Entry: e, Type: t, Related: related}))
}

func (h *Handler) section(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	term, err := h.Taxonomy.BySlug(ctx, content.VocabCategory, r.PathValue("slug"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	ids := []int64{term.ID}
	all, _ := h.Taxonomy.List(ctx, content.VocabCategory)
	// Добавляем все подрубрики: обходим дерево вширь от выбранной рубрики.
	for i := 0; i < len(ids); i++ {
		for _, c := range all {
			if c.ParentID == ids[i] {
				ids = append(ids, c.ID)
			}
		}
	}
	data, err := h.list(r, store.EntryFilter{CategoryIDs: ids}, term.Name)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.render(w, r, http.StatusOK, "list", h.view(r, Meta{Title: term.Name, Description: "Рубрика «" + term.Name + "»"}, data))
}

func (h *Handler) tag(w http.ResponseWriter, r *http.Request) {
	term, err := h.Taxonomy.BySlug(r.Context(), content.VocabTag, r.PathValue("slug"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	data, err := h.list(r, store.EntryFilter{TagID: term.ID}, "#"+term.Name)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.render(w, r, http.StatusOK, "list", h.view(r, Meta{Title: "Тег " + term.Name}, data))
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	data := ListData{Heading: "Поиск", Page: 1, Pages: 1}
	if q != "" {
		var err error
		if data, err = h.list(r, store.EntryFilter{Query: q}, "Поиск: "+q); err != nil {
			h.fail(w, r, err)
			return
		}
	}
	h.render(w, r, http.StatusOK, "search", h.view(r, Meta{Title: "Поиск"}, data))
}

type urlset struct {
	XMLName xml.Name `xml:"urlset"`
	NS      string   `xml:"xmlns,attr"`
	URLs    []sitemapURL
}

type sitemapURL struct {
	XMLName xml.Name `xml:"url"`
	Loc     string   `xml:"loc"`
	LastMod string   `xml:"lastmod,omitempty"`
}

func (h *Handler) sitemap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	set := urlset{NS: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	set.URLs = append(set.URLs, sitemapURL{Loc: h.BaseURL + "/"})
	for _, t := range h.listTypes() {
		set.URLs = append(set.URLs, sitemapURL{Loc: h.BaseURL + "/" + t.URLPrefix})
	}
	cats, _ := h.Taxonomy.List(ctx, content.VocabCategory)
	for _, c := range cats {
		set.URLs = append(set.URLs, sitemapURL{Loc: h.BaseURL + "/section/" + c.Slug})
	}
	entries, _, err := store.NewEntryRepo(h.Content.DB).List(ctx, store.EntryFilter{Status: content.StatusPublished, Sort: "published", Limit: 50000})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	for _, e := range entries {
		if t, ok := h.Content.Types.Get(e.Type); ok {
			set.URLs = append(set.URLs, sitemapURL{Loc: h.BaseURL + t.Path(e.Slug), LastMod: e.UpdatedAt.UTC().Format(time.RFC3339)})
		}
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	fmt.Fprint(w, xml.Header)
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	enc.Encode(set)
}

func (h *Handler) robots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "User-agent: *\nDisallow: /admin/\nDisallow: /search\nAllow: /\n\nSitemap: %s/sitemap.xml\n", h.BaseURL)
}
