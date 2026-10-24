package admin

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ykkssyaa/vestnik-cms/internal/access"
)

var statPeriods = []int{7, 30, 90}

func statDays(r *http.Request) int {
	n, _ := strconv.Atoi(r.URL.Query().Get("days"))
	for _, p := range statPeriods {
		if n == p {
			return n
		}
	}
	return 30
}

type statsData struct {
	Periods []int
	D       any
}

func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
	if !h.Policy.Can(current(r).Actor(), "stats.view", access.Resource{}) {
		h.forbidden(w, r)
		return
	}
	d, err := h.Stats.Build(r.Context(), statDays(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.render(w, r, http.StatusOK, "stats", h.page(w, r, "stats", "Статистика", statsData{Periods: statPeriods, D: d}))
}

// statsCSV — просмотры и публикации по дням для выгрузки в таблицу.
func (h *Handler) statsCSV(w http.ResponseWriter, r *http.Request) {
	if !h.Policy.Can(current(r).Actor(), "stats.view", access.Resource{}) {
		h.forbidden(w, r)
		return
	}
	d, err := h.Stats.Build(r.Context(), statDays(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="vestnik-stats-%s-%s.csv"`, d.From, d.To))
	cw := csv.NewWriter(w)
	cw.Write([]string{"day", "views", "visitors", "published"})
	for i, p := range d.ViewsByDay {
		cw.Write([]string{p.Day, strconv.Itoa(p.Views), strconv.Itoa(p.Visitors), strconv.Itoa(d.PublishedByDay[i].Views)})
	}
	cw.Flush()
}
