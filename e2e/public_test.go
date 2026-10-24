// Сквозные тесты публичной части (spec/05-public.md, порция 4.6): поднимается всё
// приложение, действия выполняются через HTTP так же, как в браузере.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/ykkssyaa/vestnik-cms/internal/content"
	"github.com/ykkssyaa/vestnik-cms/internal/testkit"
)

const root = ".."

var article = url.Values{
	"title": {"Мост через Неву откроют в ноябре"}, "slug": {"most-cherez-nevu"},
	"data.lead": {"Движение по новому мосту запустят после испытаний."},
	"data.body": {"Испытания **завершатся** к концу месяца.\n\n- полосы: 6\n- длина: 800 м"},
	"tags":      {"мосты, транспорт"},
}

// Сценарий 1. Автор пишет → редактор публикует → материал виден везде; снят с публикации → исчез везде.
func TestPublishFlowVisibleEverywhere(t *testing.T) {
	e := testkit.NewEnv(t, root)
	ed, author, anon := e.Login("editor@v.ru"), e.Login("author@v.ru"), e.Anon()

	ed.Post("/admin/terms", url.Values{"vocabulary": {"category"}, "name": {"Город"}, "slug": {"gorod"}})
	ed.Post("/admin/terms", url.Values{"vocabulary": {"category"}, "name": {"Транспорт"}, "slug": {"transport"}, "parent_id": {"1"}})
	form := testkit.Clone(article)
	form.Set("category_ids", "2") // подрубрика «Транспорт» внутри «Города»
	id := testkit.EntryID(t, author.Post("/admin/entries/new?type=article", form))
	author.Post("/admin/entries/"+id+"/transition", url.Values{"name": {"submit"}})

	pages := map[string]string{
		"страница материала":   "/articles/most-cherez-nevu",
		"главная":              "/",
		"лента типа":           "/articles",
		"родительская рубрика": "/section/gorod",
		"рубрика":              "/section/transport",
		"тег":                  "/tag/mosty",
		"поиск":                "/search?q=" + url.QueryEscape("НЕВУ"),
		"sitemap":              "/sitemap.xml",
		"API список":           "/api/v1/entries?category=transport",
		"API по slug":          "/api/v1/types/article/entries/most-cherez-nevu",
	}
	check := func(wantVisible bool) {
		t.Helper()
		for name, path := range pages {
			resp, body := anon.Get(path)
			visible := resp.StatusCode == 200 && (strings.Contains(body, "Мост через Неву") || strings.Contains(body, "most-cherez-nevu"))
			if visible != wantVisible {
				t.Errorf("%s (%s): виден=%v, ожидали %v (код %d)", name, path, visible, wantVisible, resp.StatusCode)
			}
		}
	}
	check(false) // на модерации — нигде не виден

	if resp := ed.Post("/admin/entries/"+id+"/transition", url.Values{"name": {"publish"}}); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("публикация: %d", resp.StatusCode)
	}
	check(true)

	// Меню собирается из контент-модели: рубрика верхнего уровня и страница с show_in_menu.
	pid := testkit.EntryID(t, ed.Post("/admin/entries/new?type=page", url.Values{"title": {"О редакции"}, "slug": {"about"},
		"data.body": {"Мы пишем о городе."}, "data.show_in_menu": {"on"}}))
	ed.Post("/admin/entries/"+pid+"/transition", url.Values{"name": {"submit"}})
	ed.Post("/admin/entries/"+pid+"/transition", url.Values{"name": {"publish"}})
	_, home := anon.Get("/")
	menu := home[strings.Index(home, `<nav class="menu">`):]
	menu = menu[:strings.Index(menu, "</nav>")]
	for _, item := range []string{`href="/section/gorod"`, `href="/about"`} {
		if !strings.Contains(menu, item) {
			t.Errorf("в меню нет %s", item)
		}
	}
	if strings.Contains(menu, `href="/section/transport"`) {
		t.Error("подрубрика не должна попадать в меню")
	}
	if resp, _ := anon.Get("/about"); resp.StatusCode != 200 {
		t.Errorf("статическая страница: %d", resp.StatusCode)
	}

	ed.Post("/admin/entries/"+id+"/transition", url.Values{"name": {"unpublish"}})
	check(false)
}

// Сценарий 2. Разметка в заголовке и тексте выводится как текст на сайте, в панели и в API.
func TestMarkupInjectionIsEscaped(t *testing.T) {
	e := testkit.NewEnv(t, root)
	ed := e.Login("editor@v.ru")
	const evilTitle = `<script>alert("xss")</script> Заголовок`
	form := testkit.Clone(article)
	form.Set("title", evilTitle)
	form.Set("data.lead", `<img src=x onerror=alert(1)>`)
	form.Set("data.body", "[ссылка](javascript:alert(1)) и <b>жирный</b>")
	form.Set("seo_description", `"><script>alert(2)</script>`)
	id := testkit.EntryID(t, ed.Post("/admin/entries/new?type=article", form))
	ed.Post("/admin/entries/"+id+"/transition", url.Values{"name": {"submit"}})
	ed.Post("/admin/entries/"+id+"/transition", url.Values{"name": {"publish"}})

	anon := e.Anon()
	for _, path := range []string{"/articles/most-cherez-nevu", "/", "/articles", "/search?q=" + url.QueryEscape("заголовок")} {
		resp, body := anon.Get(path)
		if resp.StatusCode != 200 {
			t.Fatalf("%s: %d", path, resp.StatusCode)
		}
		for _, raw := range []string{`<script>alert`, `<img src=x`, `href="javascript:`, `<b>жирный</b>`} {
			if strings.Contains(body, raw) {
				t.Errorf("%s: неэкранированная разметка %q", path, raw)
			}
		}
	}
	_, body := anon.Get("/articles/most-cherez-nevu")
	if !strings.Contains(body, "&lt;script&gt;alert(&#34;xss&#34;)&lt;/script&gt; Заголовок") {
		t.Error("заголовок должен выводиться как экранированный текст")
	}
	_, adminBody := ed.Get("/admin/entries")
	if strings.Contains(adminBody, `<script>alert("xss")`) {
		t.Error("панель выводит заголовок без экранирования")
	}
	resp, apiBody := anon.Get("/api/v1/entries/" + id)
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") || resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("API должен отдавать JSON с nosniff, получили %q", ct)
	}
	var got struct{ Title string }
	json.Unmarshal([]byte(apiBody), &got)
	if got.Title != evilTitle {
		t.Errorf("API должно отдавать данные как есть (это JSON, не HTML): %q", got.Title)
	}
}

// Сценарий 3. Администратор переключает тему — меняется разметка, наследуемые части остаются.
func TestThemeSwitch(t *testing.T) {
	e := testkit.NewEnv(t, root)
	adm, anon := e.Login("admin@v.ru"), e.Anon()
	_, body := anon.Get("/")
	if !strings.Contains(body, `class="theme-classic"`) || !strings.Contains(body, `class="site-header"`) {
		t.Fatal("по умолчанию должна быть классическая тема")
	}
	if resp := e.Login("editor@v.ru").Post("/admin/settings", url.Values{"theme": {"newspaper"}}); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("редактор не должен менять тему: %d", resp.StatusCode)
	}
	if msg := testkit.Flash(adm.Post("/admin/settings", url.Values{"theme": {"_admin"}})); !strings.Contains(msg, "Нет такой темы") {
		t.Fatalf("служебная тема панели не должна выбираться для сайта: %q", msg)
	}
	adm.Post("/admin/settings", url.Values{"theme": {"newspaper"}})
	_, body = anon.Get("/")
	if !strings.Contains(body, `class="masthead"`) || strings.Contains(body, `class="site-header"`) {
		t.Fatal("шапка газетной темы не применилась")
	}
	if !strings.Contains(body, "Сделано на собственной CMS") {
		t.Fatal("подвал должен наследоваться от классической темы")
	}
	resp, css := anon.Get("/static/newspaper/site.css")
	if resp.StatusCode != 200 || !strings.Contains(css, "masthead") {
		t.Fatalf("CSS газетной темы: %d", resp.StatusCode)
	}
	if resp, _ := anon.Get("/static/newspaper/../classic/theme.json"); resp.StatusCode == 200 {
		t.Fatal("выход из каталога темы через ..")
	}
}

// Сценарий 4. API: пагинация, ограничение per_page, фильтры по таксономии, ошибки.
func TestAPIPaginationAndFilters(t *testing.T) {
	e := testkit.NewEnv(t, root)
	ctx := context.Background()
	ed := e.Actor("editor@v.ru")
	sport := &content.Term{Vocabulary: content.VocabCategory, Name: "Спорт", Slug: "sport"}
	if err := e.App.Taxonomy.Create(ctx, ed, sport); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 25; i++ {
		in := content.Input{Title: fmt.Sprintf("Новость %02d", i), Data: map[string]any{"lead": "Коротко", "body": "Текст"}}
		if i%5 == 0 {
			in.CategoryIDs, in.Tags = []int64{sport.ID}, []string{"футбол"}
		}
		n, err := e.App.Content.Create(ctx, ed, "news", in)
		if err != nil {
			t.Fatal(err)
		}
		e.App.Content.Transition(ctx, ed, n.ID, "submit")
		e.App.Content.Transition(ctx, ed, n.ID, "publish")
	}
	type page struct {
		Items []struct{ Title string }
		Meta  struct {
			Page    int
			PerPage int `json:"per_page"` // без тега encoding/json не сопоставит per_page с PerPage
			Total   int
			Pages   int
		}
		Links struct{ Self, Next, Prev string }
	}
	get := func(path string, wantCode int) page {
		t.Helper()
		resp, body := e.Anon().Get(path)
		if resp.StatusCode != wantCode {
			t.Fatalf("%s: код %d, ожидали %d: %s", path, resp.StatusCode, wantCode, body)
		}
		var p page
		json.Unmarshal([]byte(body), &p)
		return p
	}
	p := get("/api/v1/entries?type=news&per_page=10&page=2", 200)
	if len(p.Items) != 10 || p.Meta.Total != 25 || p.Meta.Pages != 3 || p.Meta.Page != 2 {
		t.Fatalf("пагинация: %+v, элементов %d", p.Meta, len(p.Items))
	}
	if !strings.Contains(p.Links.Next, "page=3") || !strings.Contains(p.Links.Prev, "page=1") {
		t.Fatalf("ссылки next/prev: %+v", p.Links)
	}
	if last := get("/api/v1/entries?type=news&per_page=10&page=3", 200); len(last.Items) != 5 || last.Links.Next != "" {
		t.Fatalf("последняя страница: %d элементов, next=%q", len(last.Items), last.Links.Next)
	}
	if capped := get("/api/v1/entries?per_page=1000", 200); capped.Meta.PerPage != 100 {
		t.Fatalf("per_page должен ограничиваться 100: %d", capped.Meta.PerPage)
	}
	if cat := get("/api/v1/entries?category=sport", 200); cat.Meta.Total != 5 {
		t.Fatalf("фильтр по рубрике: %d", cat.Meta.Total)
	}
	if tag := get("/api/v1/entries?tag=futbol&type=news", 200); tag.Meta.Total != 5 {
		t.Fatalf("фильтр по тегу: %d", tag.Meta.Total)
	}
	get("/api/v1/entries?page=abc", 400)
	get("/api/v1/entries?type=nope", 400)
	get("/api/v1/entries?tag=nope", 404)
	get("/api/v1/entries/999999", 404)
	if resp, body := e.Anon().Get("/api/v1/types"); resp.StatusCode != 200 || !strings.Contains(body, `"url_prefix": "reviews"`) {
		t.Fatal("API типов")
	}
}

// Сценарий 5. Просмотры на сайте попадают в статистику; экран статистики — по праву stats.view.
func TestViewsReachStatsScreen(t *testing.T) {
	e := testkit.NewEnv(t, root)
	ctx := context.Background()
	ed := e.Actor("editor@v.ru")
	a, _ := e.App.Content.Create(ctx, ed, "article", content.Input{Title: "Читаемый материал", Data: map[string]any{"lead": "Л", "body": "Т"}})
	e.App.Content.Transition(ctx, ed, a.ID, "submit")
	e.App.Content.Transition(ctx, ed, a.ID, "publish")

	view := func(ua string) {
		req, _ := http.NewRequest(http.MethodGet, e.Srv.URL+"/articles/chitaemyy-material", nil)
		req.Header.Set("User-Agent", ua)
		e.Anon().Do(req)
	}
	view("Mozilla/5.0 Firefox/131.0")
	view("Mozilla/5.0 Firefox/131.0") // повтор в течение 30 минут
	view("Googlebot/2.1")
	view("Mozilla/5.0 Chrome/130.0") // другой посетитель (другой User-Agent)

	if resp, _ := e.Login("author@v.ru").Get("/admin/stats"); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("автору статистика недоступна, получили %d", resp.StatusCode)
	}
	edc := e.Login("editor@v.ru")
	resp, body := edc.Get("/admin/stats?days=7")
	if resp.StatusCode != 200 {
		t.Fatalf("статистика: %d", resp.StatusCode)
	}
	if !strings.Contains(body, "<b>2</b><span>просмотров</span>") || !strings.Contains(body, "Читаемый материал") {
		t.Fatalf("ожидали 2 просмотра (повтор и бот не считаются) и материал в топе")
	}
	resp, csv := edc.Get("/admin/stats.csv?days=7")
	if resp.StatusCode != 200 || !strings.HasPrefix(csv, "day,views,visitors,published\n") || !strings.Contains(csv, ",2,2,1") {
		t.Fatalf("CSV: %d\n%s", resp.StatusCode, csv)
	}
}

// SEO: метатеги из модели, og:image из обложки, sitemap, robots, 404.
func TestSEOAndMedia(t *testing.T) {
	e := testkit.NewEnv(t, root)
	ctx := context.Background()
	ed := e.Actor("editor@v.ru")
	var buf bytes.Buffer
	jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1800, 900)), nil)
	m, err := e.App.Media.Upload(ctx, ed, "cover.jpg", &buf)
	if err != nil {
		t.Fatal(err)
	}
	a, err := e.App.Content.Create(ctx, ed, "article", content.Input{Title: "Новая набережная", SEOTitle: "Набережная: что изменится",
		Data: map[string]any{"lead": "Набережную откроют летом.", "body": "Текст", "cover": fmt.Sprint(m.ID)}})
	if err != nil {
		t.Fatal(err)
	}
	e.App.Content.Transition(ctx, ed, a.ID, "submit")
	e.App.Content.Transition(ctx, ed, a.ID, "publish")

	anon := e.Anon()
	_, body := anon.Get("/articles/novaya-naberezhnaya")
	for _, want := range []string{
		"<title>Набережная: что изменится — Вестник</title>",
		`<meta name="description" content="Набережную откроют летом.">`,
		`<link rel="canonical" href="http://test.local/articles/novaya-naberezhnaya">`,
		`<meta property="og:image" content="http://test.local/uploads/` + m.Variants["large"].Path + `">`,
		`<meta property="og:type" content="article">`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("на странице нет %s", want)
		}
	}
	// Главная и ленты с обложками: ветки шаблонов с изображениями тоже должны рендериться в обеих темах.
	for _, theme := range []string{"classic", "newspaper"} {
		e.App.DB.Exec(`INSERT INTO settings (key, value, updated_at) VALUES ('theme', ?, '') ON CONFLICT(key) DO UPDATE SET value = excluded.value`, theme)
		for _, p := range []string{"/", "/articles", "/articles/novaya-naberezhnaya"} {
			resp, body := anon.Get(p)
			wantImage := theme == "classic" || p != "/articles" // карточки газетной темы — без картинок, это её стиль
			if resp.StatusCode != 200 || strings.Contains(body, "/uploads/") != wantImage {
				t.Errorf("%s %s: код %d, изображение в разметке: %v", theme, p, resp.StatusCode, strings.Contains(body, "/uploads/"))
			}
		}
	}
	if resp, _ := anon.Get("/uploads/" + m.Variants["medium"].Path);resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "image/jpeg" {
		t.Errorf("вариант изображения не отдаётся: %d", resp.StatusCode)
	}
	dir := m.Path[:strings.LastIndex(m.Path, "/")+1]
	if resp, _ := anon.Get("/uploads/" + dir); resp.StatusCode == 200 {
		t.Error("листинг каталога загрузок открыт")
	}
	_, sm := anon.Get("/sitemap.xml")
	if !strings.Contains(sm, "<loc>http://test.local/articles/novaya-naberezhnaya</loc>") || !strings.Contains(sm, "<lastmod>") {
		t.Errorf("sitemap: %s", sm)
	}
	_, robots := anon.Get("/robots.txt")
	if !strings.Contains(robots, "Disallow: /admin/") || !strings.Contains(robots, "Sitemap: http://test.local/sitemap.xml") {
		t.Errorf("robots.txt: %s", robots)
	}
	if resp, body := anon.Get("/articles/net-takogo"); resp.StatusCode != 404 || !strings.Contains(body, "Страница не найдена") {
		t.Errorf("404: %d", resp.StatusCode)
	}
}
