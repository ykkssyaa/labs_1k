// Package api — REST API для headless-режима: только опубликованные материалы, JSON.
package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/ykkssyaa/vestnik-cms/internal/content"
	"github.com/ykkssyaa/vestnik-cms/internal/store"
	"github.com/ykkssyaa/vestnik-cms/internal/web"
)

const maxPerPage = 100

type Handler struct {
	Content  *content.Service
	Taxonomy *content.Taxonomy
	Media    web.MediaLookup
	BaseURL  string
	Log      *slog.Logger
}

func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/types", h.types)
	mux.HandleFunc("GET /api/v1/entries", h.entries)
	mux.HandleFunc("GET /api/v1/entries/{id}", h.entryByID)
	mux.HandleFunc("GET /api/v1/types/{type}/entries/{slug}", h.entryBySlug)
	mux.HandleFunc("GET /api/v1/terms", h.terms)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// MediaJSON — поле типа media в ответе API: адреса оригинала и вариантов.
type MediaJSON struct {
	ID       int64             `json:"id"`
	Alt      string            `json:"alt"`
	Width    int               `json:"width"`
	Height   int               `json:"height"`
	URL      string            `json:"url"`
	Variants map[string]string `json:"variants"`
}

// EntryJSON — материал в ответе API.
type EntryJSON struct {
	ID             int64          `json:"id"`
	Type           string         `json:"type"`
	Title          string         `json:"title"`
	Slug           string         `json:"slug"`
	URL            string         `json:"url"`
	Author         string         `json:"author"`
	SEOTitle       string         `json:"seo_title,omitempty"`
	SEODescription string         `json:"seo_description,omitempty"`
	Data           map[string]any `json:"data"`
	Categories     []TermJSON     `json:"categories"`
	Tags           []TermJSON     `json:"tags"`
	PublishedAt    *time.Time     `json:"published_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type TermJSON struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	ParentID int64  `json:"parent_id,omitempty"`
}

func termsJSON(terms []store.Term) []TermJSON {
	out := make([]TermJSON, 0, len(terms))
	for _, t := range terms {
		out = append(out, TermJSON{ID: t.ID, Name: t.Name, Slug: t.Slug, ParentID: t.ParentID})
	}
	return out
}

func (h *Handler) toJSON(r *http.Request, e *content.Entry) EntryJSON {
	t, _ := h.Content.Types.Get(e.Type)
	data := map[string]any{}
	for k, v := range e.Data {
		data[k] = v
	}
	if t != nil {
		for _, f := range t.Fields {
			if f.Type != content.FieldMedia {
				continue
			}
			if m := h.Media(r.Context(), web.MediaID(e.Data[f.Name])); m != nil {
				mj := MediaJSON{ID: m.ID, Alt: m.Alt, Width: m.Width, Height: m.Height,
					URL: h.BaseURL + web.MediaURL(m, ""), Variants: map[string]string{}}
				for name := range m.Variants {
					mj.Variants[name] = h.BaseURL + web.MediaURL(m, name)
				}
				data[f.Name] = mj
			}
		}
	}
	path := "/"
	if t != nil {
		path = t.Path(e.Slug)
	}
	return EntryJSON{ID: e.ID, Type: e.Type, Title: e.Title, Slug: e.Slug, URL: h.BaseURL + path, Author: e.AuthorName,
		SEOTitle: e.SEOTitle, SEODescription: e.SEODescription, Data: data,
		Categories: termsJSON(e.Categories), Tags: termsJSON(e.Tags), PublishedAt: e.PublishedAt, UpdatedAt: e.UpdatedAt}
}

func (h *Handler) types(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": h.Content.Types.List()})
}

type listMeta struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
	Total   int `json:"total"`
	Pages   int `json:"pages"`
}

type listLinks struct {
	Self string `json:"self"`
	Next string `json:"next,omitempty"`
	Prev string `json:"prev,omitempty"`
}

func intParam(q url.Values, key string, def int) (int, error) {
	s := q.Get(key)
	if s == "" {
		return def, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, errors.New("параметр " + key + " должен быть целым числом ≥ 1")
	}
	return n, nil
}

func (h *Handler) entries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, err := intParam(q, "page", 1)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	per, err := intParam(q, "per_page", 20)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	per = min(per, maxPerPage)
	f := store.EntryFilter{Status: content.StatusPublished, Sort: "published", Query: q.Get("q"), Limit: per, Offset: (page - 1) * per}
	if typ := q.Get("type"); typ != "" {
		if _, ok := h.Content.Types.Get(typ); !ok {
			writeError(w, http.StatusBadRequest, "неизвестный тип "+typ)
			return
		}
		f.Type = typ
	}
	for _, p := range []struct {
		param, vocab string
		dst          *int64
	}{{"category", content.VocabCategory, &f.CategoryID}, {"tag", content.VocabTag, &f.TagID}} {
		slug := q.Get(p.param)
		if slug == "" {
			continue
		}
		term, err := h.Taxonomy.BySlug(r.Context(), p.vocab, slug)
		if err != nil {
			writeError(w, http.StatusNotFound, "термин "+slug+" не найден")
			return
		}
		*p.dst = term.ID
	}
	list, total, err := h.Content.List(r.Context(), f)
	if err != nil {
		h.Log.Error("api entries", "err", err)
		writeError(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	items := make([]EntryJSON, 0, len(list))
	for _, e := range list {
		items = append(items, h.toJSON(r, e))
	}
	pages := max((total+per-1)/per, 1)
	link := func(p int) string {
		qq := url.Values{}
		for k, v := range q {
			qq[k] = v
		}
		qq.Set("page", strconv.Itoa(p))
		qq.Set("per_page", strconv.Itoa(per))
		return h.BaseURL + "/api/v1/entries?" + qq.Encode()
	}
	links := listLinks{Self: link(page)}
	if page < pages {
		links.Next = link(page + 1)
	}
	if page > 1 {
		links.Prev = link(page - 1)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "meta": listMeta{Page: page, PerPage: per, Total: total, Pages: pages}, "links": links,
	})
}

func (h *Handler) published(w http.ResponseWriter, r *http.Request, e *content.Entry, err error) {
	if err != nil || e.Status != content.StatusPublished {
		writeError(w, http.StatusNotFound, "материал не найден")
		return
	}
	writeJSON(w, http.StatusOK, h.toJSON(r, e))
}

func (h *Handler) entryByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "id должен быть числом")
		return
	}
	e, err := h.Content.Get(r.Context(), id)
	h.published(w, r, e, err)
}

func (h *Handler) entryBySlug(w http.ResponseWriter, r *http.Request) {
	e, err := h.Content.GetBySlug(r.Context(), r.PathValue("type"), r.PathValue("slug"))
	h.published(w, r, e, err)
}

func (h *Handler) terms(w http.ResponseWriter, r *http.Request) {
	vocab := r.URL.Query().Get("vocabulary")
	if vocab == "" {
		vocab = content.VocabCategory
	}
	if vocab != content.VocabCategory && vocab != content.VocabTag {
		writeError(w, http.StatusBadRequest, "vocabulary: category или tag")
		return
	}
	terms, err := h.Taxonomy.List(r.Context(), vocab)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": termsJSON(terms)})
}
