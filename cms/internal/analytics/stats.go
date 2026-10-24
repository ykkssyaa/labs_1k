package analytics

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/ykkssyaa/vestnik-cms/internal/store"
)

// RawRetention — сколько хранить сырые просмотры после свёртки.
const RawRetention = 90 * 24 * time.Hour

type Stats struct {
	DB  *sql.DB
	Now func() time.Time
}

func NewStats(db *sql.DB) *Stats {
	return &Stats{DB: db, Now: func() time.Time { return time.Now().UTC() }}
}

func (s *Stats) today() string { return s.Now().In(Moscow).Format("2006-01-02") }

// Rollup сворачивает сырые просмотры завершённых дней в дневные таблицы.
// Значения пересчитываются целиком из сырых данных, поэтому повторный запуск ничего не удваивает.
func (s *Stats) Rollup(ctx context.Context) (days int, err error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	today := s.today()
	dayExpr := `date(created_at, '` + DayOffset + `')`
	if _, err := tx.ExecContext(ctx, `INSERT INTO metrics_daily (day, entry_id, views, visitors)
		SELECT `+dayExpr+` AS d, entry_id, COUNT(*), COUNT(DISTINCT visitor) FROM page_views
		WHERE `+dayExpr+` < ? GROUP BY d, entry_id
		ON CONFLICT(day, entry_id) DO UPDATE SET views = excluded.views, visitors = excluded.visitors`, today); err != nil {
		return 0, fmt.Errorf("rollup entries: %w", err)
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO metrics_site_daily (day, views, visitors)
		SELECT `+dayExpr+` AS d, COUNT(*), COUNT(DISTINCT visitor) FROM page_views
		WHERE `+dayExpr+` < ? GROUP BY d
		ON CONFLICT(day) DO UPDATE SET views = excluded.views, visitors = excluded.visitors`, today)
	if err != nil {
		return 0, fmt.Errorf("rollup site: %w", err)
	}
	n, _ := res.RowsAffected()
	cutoff := s.Now().Add(-RawRetention).Format(store.TimeFormat)
	if _, err := tx.ExecContext(ctx, `DELETE FROM page_views WHERE created_at < ?`, cutoff); err != nil {
		return 0, err
	}
	return int(n), tx.Commit()
}

// DayPoint — значение за день.
type DayPoint struct {
	Day      string
	Views    int
	Visitors int
	Share    int // доля от максимума периода, % — для столбцов на экране
}

// TopEntry — материал в рейтинге просмотров.
type TopEntry struct {
	ID       int64
	Title    string
	Type     string
	Views    int
	Visitors int
}

// TypeCount — публикации по типу за период.
type TypeCount struct {
	Type  string
	Count int
}

// AuthorActivity — что сделал пользователь за период.
type AuthorActivity struct {
	UserID    int64
	Name      string
	Created   int
	Edits     int
	Submitted int
	Published int // опубликовано его материалов
}

// Dashboard — всё, что показывает экран статистики.
type Dashboard struct {
	Days             int
	From, To         string
	TotalViews       int
	TotalVisitors    int
	Published        int
	ActiveAuthors    int
	ModerationMedian time.Duration
	Moderated        int
	ViewsByDay       []DayPoint
	PublishedByDay   []DayPoint
	Top              []TopEntry
	ByType           []TypeCount
	Authors          []AuthorActivity
}

// unrolled — подзапрос с сырыми просмотрами дней, ещё не попавших в свёртку (обычно сегодня).
const unrolled = `SELECT date(created_at, '` + DayOffset + `') AS day, entry_id, visitor FROM page_views
	WHERE date(created_at, '` + DayOffset + `') >= ? AND date(created_at, '` + DayOffset + `') NOT IN (SELECT day FROM metrics_site_daily)`

// Build считает показатели за последние days дней, включая сегодня.
func (s *Stats) Build(ctx context.Context, days int) (*Dashboard, error) {
	now := s.Now().In(Moscow)
	d := &Dashboard{Days: days, To: now.Format("2006-01-02"), From: now.AddDate(0, 0, -(days - 1)).Format("2006-01-02")}
	steps := []func(context.Context, *Dashboard) error{s.viewsByDay, s.top, s.publications, s.authors, s.moderation}
	for _, step := range steps {
		if err := step(ctx, d); err != nil {
			return nil, err
		}
	}
	return d, nil
}

// fillDays дополняет ряд нулями за дни без данных и считает доли для столбцов.
func fillDays(from string, days int, values map[string]DayPoint) []DayPoint {
	start, _ := time.Parse("2006-01-02", from)
	out := make([]DayPoint, days)
	peak := 0
	for i := range out {
		day := start.AddDate(0, 0, i).Format("2006-01-02")
		p := values[day]
		p.Day = day
		out[i] = p
		peak = max(peak, p.Views)
	}
	for i := range out {
		if peak > 0 {
			out[i].Share = out[i].Views * 100 / peak
		}
	}
	return out
}

func (s *Stats) viewsByDay(ctx context.Context, d *Dashboard) error {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT day, views, visitors FROM metrics_site_daily WHERE day BETWEEN ? AND ?
		UNION ALL
		SELECT day, COUNT(*), COUNT(DISTINCT visitor) FROM (`+unrolled+`) GROUP BY day`, d.From, d.To, d.From)
	if err != nil {
		return fmt.Errorf("views by day: %w", err)
	}
	defer rows.Close()
	values := map[string]DayPoint{}
	for rows.Next() {
		var p DayPoint
		if err := rows.Scan(&p.Day, &p.Views, &p.Visitors); err != nil {
			return err
		}
		values[p.Day] = p
		d.TotalViews += p.Views
		d.TotalVisitors += p.Visitors // сумма дневных уникальных: посетитель разных дней неотличим (соль дня)
	}
	d.ViewsByDay = fillDays(d.From, d.Days, values)
	return rows.Err()
}

func (s *Stats) top(ctx context.Context, d *Dashboard) error {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT e.id, e.title, e.type, SUM(v.views) AS views, SUM(v.visitors) FROM (
			SELECT entry_id, views, visitors FROM metrics_daily WHERE day BETWEEN ? AND ?
			UNION ALL
			SELECT entry_id, COUNT(*), COUNT(DISTINCT visitor) FROM (`+unrolled+`) GROUP BY day, entry_id
		) v JOIN entries e ON e.id = v.entry_id
		GROUP BY e.id ORDER BY views DESC, e.id LIMIT 10`, d.From, d.To, d.From)
	if err != nil {
		return fmt.Errorf("top: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var t TopEntry
		if err := rows.Scan(&t.ID, &t.Title, &t.Type, &t.Views, &t.Visitors); err != nil {
			return err
		}
		d.Top = append(d.Top, t)
	}
	return rows.Err()
}

// periodStart — начало периода в UTC в формате хранения (сутки московские).
func periodStart(from string) string {
	t, _ := time.ParseInLocation("2006-01-02", from, Moscow)
	return t.UTC().Format(store.TimeFormat)
}

func (s *Stats) publications(ctx context.Context, d *Dashboard) error {
	rows, err := s.DB.QueryContext(ctx, `SELECT date(published_at, '`+DayOffset+`') AS day, type, COUNT(*)
		FROM entries WHERE published_at >= ? GROUP BY day, type`, periodStart(d.From))
	if err != nil {
		return fmt.Errorf("publications: %w", err)
	}
	defer rows.Close()
	byDay := map[string]DayPoint{}
	byType := map[string]int{}
	for rows.Next() {
		var day, typ string
		var n int
		if err := rows.Scan(&day, &typ, &n); err != nil {
			return err
		}
		p := byDay[day]
		p.Views += n // для ряда публикаций «Views» — число публикаций за день
		byDay[day] = p
		byType[typ] += n
		d.Published += n
	}
	d.PublishedByDay = fillDays(d.From, d.Days, byDay)
	for t, n := range byType {
		d.ByType = append(d.ByType, TypeCount{Type: t, Count: n})
	}
	sort.Slice(d.ByType, func(i, j int) bool { return d.ByType[i].Count > d.ByType[j].Count })
	return rows.Err()
}

func (s *Stats) authors(ctx context.Context, d *Dashboard) error {
	since := periodStart(d.From)
	rows, err := s.DB.QueryContext(ctx, `
		SELECT u.id, u.name,
			(SELECT COUNT(*) FROM audit_log a WHERE a.user_id = u.id AND a.action = 'entry.create' AND a.created_at >= ?),
			(SELECT COUNT(*) FROM audit_log a WHERE a.user_id = u.id AND a.action = 'entry.update' AND a.created_at >= ?),
			(SELECT COUNT(*) FROM audit_log a WHERE a.user_id = u.id AND a.action = 'entry.submit' AND a.created_at >= ?),
			(SELECT COUNT(*) FROM entries e WHERE e.author_id = u.id AND e.published_at >= ?)
		FROM users u ORDER BY u.name`, since, since, since, since)
	if err != nil {
		return fmt.Errorf("authors: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var a AuthorActivity
		if err := rows.Scan(&a.UserID, &a.Name, &a.Created, &a.Edits, &a.Submitted, &a.Published); err != nil {
			return err
		}
		if a.Created+a.Edits+a.Submitted+a.Published == 0 {
			continue
		}
		d.Authors = append(d.Authors, a)
	}
	d.ActiveAuthors = len(d.Authors)
	sort.SliceStable(d.Authors, func(i, j int) bool { return d.Authors[i].Published > d.Authors[j].Published })
	return rows.Err()
}

// moderation — медиана времени от последней отправки на модерацию до публикации.
func (s *Stats) moderation(ctx context.Context, d *Dashboard) error {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT p.created_at, (SELECT s.created_at FROM audit_log s WHERE s.action = 'entry.submit'
			AND s.object_id = p.object_id AND s.id < p.id ORDER BY s.id DESC LIMIT 1)
		FROM audit_log p WHERE p.action = 'entry.publish' AND p.created_at >= ?`, periodStart(d.From))
	if err != nil {
		return fmt.Errorf("moderation: %w", err)
	}
	defer rows.Close()
	var waits []time.Duration
	for rows.Next() {
		var published string
		var submitted sql.NullString
		if err := rows.Scan(&published, &submitted); err != nil {
			return err
		}
		if submitted.Valid {
			waits = append(waits, store.ParseTime(published).Sub(store.ParseTime(submitted.String)))
		}
	}
	d.Moderated = len(waits)
	if len(waits) > 0 {
		sort.Slice(waits, func(i, j int) bool { return waits[i] < waits[j] })
		d.ModerationMedian = waits[len(waits)/2]
		if len(waits)%2 == 0 {
			d.ModerationMedian = (waits[len(waits)/2-1] + waits[len(waits)/2]) / 2
		}
	}
	return rows.Err()
}
