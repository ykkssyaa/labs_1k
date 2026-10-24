package seed

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/ykkssyaa/vestnik-cms/internal/app"
	"github.com/ykkssyaa/vestnik-cms/internal/store"
)

func TestSeedFillsEmptyDatabaseOnce(t *testing.T) {
	cfg := app.TestConfig("../..")
	cfg.UploadsDir = t.TempDir()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	a, err := app.New(cfg, log)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	ctx := context.Background()
	if err := Run(ctx, a, "", "", log); err != nil {
		t.Fatal(err)
	}
	count := func(q string) int {
		var n int
		a.DB.QueryRow(q).Scan(&n)
		return n
	}
	entries, types, views := count(`SELECT COUNT(*) FROM entries`), count(`SELECT COUNT(DISTINCT type) FROM entries`), count(`SELECT COUNT(*) FROM page_views`)
	if entries != len(items) || types != len(a.Types.List()) || views == 0 {
		t.Fatalf("материалов %d, типов %d, просмотров %d", entries, types, views)
	}
	for _, st := range []string{"draft", "review", "published", "archived"} {
		if count(`SELECT COUNT(*) FROM entries WHERE status = '`+st+`'`) == 0 {
			t.Errorf("нет материалов в статусе %s — демонстрация жизненного цикла неполная", st)
		}
	}
	if count(`SELECT COUNT(*) FROM metrics_site_daily`) == 0 {
		t.Error("статистика не свёрнута")
	}
	if store.Now() < "2026" { // часы восстановлены после заполнения
		t.Error("store.Now не восстановлен")
	}
	if err := Run(ctx, a, "", "", log); err != nil {
		t.Fatal(err)
	}
	if again := count(`SELECT COUNT(*) FROM entries`); again != entries {
		t.Fatalf("повторный seed добавил данные: %d -> %d", entries, again)
	}
}
