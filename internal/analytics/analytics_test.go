package analytics

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ykkssyaa/vestnik-cms/internal/access"
	"github.com/ykkssyaa/vestnik-cms/internal/auth"
	"github.com/ykkssyaa/vestnik-cms/internal/content"
	"github.com/ykkssyaa/vestnik-cms/internal/store"
)

type fixture struct {
	db      *sql.DB
	content *content.Service
	tracker *Tracker
	stats   *Stats
	actor   access.Actor
	clock   time.Time
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	reg, err := content.LoadRegistry("../../content_types")
	if err != nil {
		t.Fatal(err)
	}
	policy, _ := access.LoadPolicy("../../config/roles.json")
	authSvc := auth.NewService(db, policy)
	u, err := authSvc.CreateUser(context.Background(), access.Actor{}, auth.UserInput{Email: "e@v.ru", Name: "Редактор", Role: "editor", Password: "password1"})
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{db: db, actor: access.Actor{ID: u.ID, Name: u.Name, Role: u.Role},
		clock: time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)}
	f.content = content.NewService(db, reg, policy)
	f.content.Audit = authSvc
	now := func() time.Time { return f.clock }
	f.content.Now = now
	f.tracker, err = NewTracker(context.Background(), db, "http://vestnik.local")
	if err != nil {
		t.Fatal(err)
	}
	f.tracker.Now = now
	f.stats = &Stats{DB: db, Now: now}
	// Журнал действий берёт время из store.Now — подменяем, чтобы события ложились в нужные дни.
	old := store.Now
	store.Now = func() string { return f.clock.Format(store.TimeFormat) }
	t.Cleanup(func() { store.Now = old })
	return f
}

func (f *fixture) publish(t *testing.T, typ, title string, data map[string]any, wait time.Duration) *content.Entry {
	t.Helper()
	ctx := context.Background()
	e, err := f.content.Create(ctx, f.actor, typ, content.Input{Title: title, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	f.content.Transition(ctx, f.actor, e.ID, "submit")
	f.clock = f.clock.Add(wait)
	if e, err = f.content.Transition(ctx, f.actor, e.ID, "publish"); err != nil {
		t.Fatal(err)
	}
	return e
}

func article() map[string]any { return map[string]any{"lead": "Лид", "body": "Текст"} }

func visit(ip, ua string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/articles/x", nil)
	r.RemoteAddr = ip + ":51000"
	r.Header.Set("User-Agent", ua)
	return r
}

const browser = "Mozilla/5.0 (X11; Linux x86_64) Firefox/131.0"

func TestTrackSkipsBotsPrefetchAndRepeats(t *testing.T) {
	f := newFixture(t)
	e := f.publish(t, "article", "Материал", article(), time.Minute)
	ctx := context.Background()
	cases := []struct {
		name string
		req  *http.Request
		gap  time.Duration
		want bool
	}{
		{"first visit", visit("10.0.0.1", browser), 0, true},
		{"reload within 30 min", visit("10.0.0.1", browser), 10 * time.Minute, false},
		{"same reader after 30 min", visit("10.0.0.1", browser), 31 * time.Minute, true},
		{"another reader", visit("10.0.0.2", browser), 0, true},
		{"googlebot", visit("10.0.0.3", "Mozilla/5.0 (compatible; Googlebot/2.1)"), 0, false},
		{"curl", visit("10.0.0.4", "curl/8.5"), 0, false},
		{"empty user agent", visit("10.0.0.5", ""), 0, false},
	}
	for _, c := range cases {
		f.clock = f.clock.Add(c.gap)
		got, err := f.tracker.Track(ctx, c.req, e.ID)
		if err != nil || got != c.want {
			t.Errorf("%s: засчитан=%v, ожидали %v (%v)", c.name, got, c.want, err)
		}
	}
	prefetch := visit("10.0.0.6", browser)
	prefetch.Header.Set("Sec-Purpose", "prefetch")
	if ok, _ := f.tracker.Track(ctx, prefetch, e.ID); ok {
		t.Error("предзагрузка браузера засчитана как просмотр")
	}
}

func TestVisitorIsAnonymousAndDaily(t *testing.T) {
	f := newFixture(t)
	r := visit("192.168.7.42", browser)
	day1 := f.tracker.Visitor(r, f.clock)
	if day1 != f.tracker.Visitor(r, f.clock.Add(time.Hour)) {
		t.Error("в пределах дня посетитель должен совпадать")
	}
	if day1 == f.tracker.Visitor(r, f.clock.Add(24*time.Hour)) {
		t.Error("на следующий день хэш должен меняться")
	}
	e := f.publish(t, "article", "М", article(), 0)
	f.tracker.Track(context.Background(), r, e.ID)
	var dump string
	f.db.QueryRow(`SELECT visitor || '|' || referrer_host FROM page_views`).Scan(&dump)
	if strings.Contains(dump, "192.168") || strings.Contains(dump, "Firefox") {
		t.Fatalf("в БД попали персональные данные: %q", dump)
	}
}

func TestReferrerHostOnlyExternal(t *testing.T) {
	f := newFixture(t)
	e := f.publish(t, "article", "М", article(), 0)
	for ip, ref := range map[string]string{"1.1.1.1": "https://yandex.ru/search?q=секрет", "1.1.1.2": "http://vestnik.local/"} {
		r := visit(ip, browser)
		r.Header.Set("Referer", ref)
		f.tracker.Track(context.Background(), r, e.ID)
	}
	rows, _ := f.db.Query(`SELECT referrer_host FROM page_views ORDER BY referrer_host`)
	var hosts []string
	for rows.Next() {
		var h string
		rows.Scan(&h)
		hosts = append(hosts, h)
	}
	rows.Close()
	if strings.Join(hosts, ",") != ",yandex.ru" {
		t.Fatalf("источники: %v (строка запроса и внутренние переходы не хранятся)", hosts)
	}
}

// Свёртка идемпотентна и не меняет итогов: до неё, после неё и после повторного запуска числа одни и те же.
func TestRollupIsIdempotent(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a := f.publish(t, "article", "А", article(), time.Minute)
	b := f.publish(t, "news", "Б", article(), time.Minute)
	// 3 дня: 18, 19 и 20 октября (сегодня). Читатель 1 читает оба материала — один уникальный посетитель сайта.
	f.clock = time.Date(2026, 10, 18, 10, 0, 0, 0, time.UTC)
	f.tracker.Track(ctx, visit("1.1.1.1", browser), a.ID)
	f.tracker.Track(ctx, visit("1.1.1.1", browser), b.ID)
	f.tracker.Track(ctx, visit("1.1.1.2", browser), a.ID)
	f.clock = time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	f.tracker.Track(ctx, visit("1.1.1.1", browser), a.ID)
	f.clock = time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	f.tracker.Track(ctx, visit("1.1.1.3", browser), b.ID)

	before, err := f.stats.Build(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		days, err := f.stats.Rollup(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if days != 2 { // сегодняшний день не сворачивается
			t.Fatalf("свёрнуто дней: %d", days)
		}
		after, _ := f.stats.Build(ctx, 7)
		if after.TotalViews != before.TotalViews || after.TotalVisitors != before.TotalVisitors || after.Top[0] != before.Top[0] {
			t.Fatalf("запуск %d: итоги изменились: %d/%d -> %d/%d", i+1, before.TotalViews, before.TotalVisitors, after.TotalViews, after.TotalVisitors)
		}
	}
	if before.TotalViews != 5 || before.TotalVisitors != 4 { // 18-е: 3 просмотра, 2 посетителя; 19-е: 1/1; 20-е: 1/1
		t.Fatalf("итоги: просмотров %d, уникальных %d", before.TotalViews, before.TotalVisitors)
	}
	var site18 int
	f.db.QueryRow(`SELECT visitors FROM metrics_site_daily WHERE day = '2026-10-18'`).Scan(&site18)
	if site18 != 2 {
		t.Fatalf("уникальные сайта за 18-е: %d (не сумма по материалам)", site18)
	}
	if before.Top[0].ID != a.ID || before.Top[0].Views != 3 {
		t.Fatalf("топ: %+v", before.Top)
	}
	last := before.ViewsByDay[len(before.ViewsByDay)-1]
	if last.Day != "2026-10-20" || last.Views != 1 || len(before.ViewsByDay) != 7 {
		t.Fatalf("ряд по дням: %+v", before.ViewsByDay)
	}
}

func TestPublicationsAuthorsAndModeration(t *testing.T) {
	f := newFixture(t)
	f.publish(t, "article", "А1", article(), 2*time.Hour)
	f.publish(t, "article", "А2", article(), 4*time.Hour)
	f.publish(t, "news", "Н1", article(), 6*time.Hour)
	d, err := f.stats.Build(context.Background(), 30)
	if err != nil {
		t.Fatal(err)
	}
	if d.Published != 3 || len(d.ByType) != 2 || d.ByType[0].Type != "article" || d.ByType[0].Count != 2 {
		t.Fatalf("публикации: %d, по типам %+v", d.Published, d.ByType)
	}
	if d.ModerationMedian != 4*time.Hour || d.Moderated != 3 {
		t.Fatalf("медиана модерации %v по %d материалам, ожидали 4h по 3", d.ModerationMedian, d.Moderated)
	}
	if len(d.Authors) != 1 || d.Authors[0].Created != 3 || d.Authors[0].Submitted != 3 || d.Authors[0].Published != 3 {
		t.Fatalf("активность авторов: %+v", d.Authors)
	}
}
