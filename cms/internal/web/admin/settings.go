package admin

import (
	"net/http"

	"github.com/ykkssyaa/vestnik-cms/internal/access"
	"github.com/ykkssyaa/vestnik-cms/internal/render"
)

type settingsData struct {
	Themes []*render.Theme
	Active string
}

func (h *Handler) settings(w http.ResponseWriter, r *http.Request) {
	if !h.Policy.Can(current(r).Actor(), "settings.manage", access.Resource{}) {
		h.forbidden(w, r)
		return
	}
	h.render(w, r, http.StatusOK, "settings", h.page(w, r, "settings", "Настройки сайта",
		settingsData{Themes: h.Render.Themes(), Active: h.ActiveTheme(r.Context())}))
}

func (h *Handler) saveSettings(w http.ResponseWriter, r *http.Request) {
	cur := current(r)
	if !h.Policy.Can(cur.Actor(), "settings.manage", access.Resource{}) {
		h.forbidden(w, r)
		return
	}
	theme := r.FormValue("theme")
	if !h.Render.HasTheme(theme) {
		h.redirect(w, r, "/admin/settings", "Нет такой темы")
		return
	}
	if err := h.Settings.Set(r.Context(), "theme", theme); err != nil {
		h.fail(w, r, err)
		return
	}
	h.Auth.Record(r.Context(), h.Content.DB, cur.Actor(), "settings.theme", "settings", 0, theme)
	h.redirect(w, r, "/admin/settings", "Тема сайта: "+theme)
}
