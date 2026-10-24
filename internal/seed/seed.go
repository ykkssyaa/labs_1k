// Package seed заполняет пустую БД демонстрационными данными для стенда.
package seed

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"log/slog"
	"math"
	"math/rand/v2"
	"time"

	"github.com/ykkssyaa/vestnik-cms/internal/access"
	"github.com/ykkssyaa/vestnik-cms/internal/app"
	"github.com/ykkssyaa/vestnik-cms/internal/auth"
	"github.com/ykkssyaa/vestnik-cms/internal/content"
	"github.com/ykkssyaa/vestnik-cms/internal/store"
)

// Демо-учётки. Пароль администратора берётся из окружения, если задан.
const (
	EditorEmail = "editor@vestnik.local"
	AuthorEmail = "author@vestnik.local"
	DemoPass    = "demo-password"
)

type item struct {
	typ, title, slug string
	data             map[string]any
	cats             []string
	tags             []string
	daysAgo          int    // когда опубликован
	status           string // итоговый статус
	byAuthor         bool
	cover            bool
	popularity       float64
}

const lorem = "Город меняется быстрее, чем успевают обновляться карты. " +
	"Редакция «Вестника» разбиралась, что это значит для жителей и кто за это платит."

func body(extra string) string {
	return lorem + "\n\n" + extra + "\n\n## Что дальше\n\n- обсуждение с жителями\n- экспертиза проекта\n- решение до конца года\n\n" +
		"> Мы хотим, чтобы изменения были видны каждому — сказал представитель администрации.\n\nПодробности — в [документах проекта](https://example.org/project)."
}

var items = []item{
	{typ: "article", title: "Как изменится набережная после реконструкции", slug: "naberezhnaya-rekonstrukciya",
		data: map[string]any{"lead": "Пешеходная зона станет вдвое шире, а парковки уйдут под землю.", "body": body("Проект обсуждали **два года**: жители спорили о деревьях, архитекторы — о высоте фонарей."), "reading_time": "7"},
		cats: []string{"gorod"}, tags: []string{"благоустройство", "набережная"}, daysAgo: 1, status: "published", cover: true, popularity: 1},
	{typ: "news", title: "Новый мост через реку откроют в ноябре", slug: "most-otkroyut-v-noyabre",
		data: map[string]any{"lead": "Движение по мосту запустят после испытаний нагрузкой.", "body": body("Испытания *завершатся* к концу месяца."), "source_url": "https://example.org/most", "is_breaking": "on"},
		cats: []string{"transport"}, tags: []string{"мосты", "транспорт"}, daysAgo: 0, status: "published", popularity: 0.9},
	{typ: "news", title: "В центре закроют две улицы на выходные", slug: "zakroyut-ulitsy",
		data: map[string]any{"lead": "Ограничения связаны с городским фестивалем.", "body": body("Объезд будет организован по соседним кварталам.")},
		cats: []string{"transport"}, tags: []string{"транспорт", "фестиваль"}, daysAgo: 3, status: "published", byAuthor: true, popularity: 0.6},
	{typ: "review", title: "«Чайка» в Молодёжном театре: Чехов без иронии", slug: "chayka-molodezhnyy",
		data: map[string]any{"subject": "«Чайка», реж. А. Петров", "subject_kind": "спектакль", "rating": "8", "body": body("Режиссёр убирает привычную иронию и оставляет героев один на один с залом.")},
		cats: []string{"teatr"}, tags: []string{"театр", "чехов"}, daysAgo: 5, status: "published", byAuthor: true, cover: true, popularity: 0.5},
	{typ: "review", title: "Выставка «Город света» в Манеже", slug: "vystavka-gorod-sveta",
		data: map[string]any{"subject": "«Город света»", "subject_kind": "выставка", "rating": "7", "body": body("Свет здесь — главный материал: от проекций до старых фонарей.")},
		cats: []string{"kultura"}, tags: []string{"выставки"}, daysAgo: 9, status: "published", cover: true, popularity: 0.4},
	{typ: "review", title: "Кафе «Пышечная №1»: классика без сюрпризов", slug: "pyshechnaya",
		data: map[string]any{"subject": "Пышечная №1", "subject_kind": "ресторан", "rating": "9", "body": body("Пышки по-прежнему горячие, очередь по-прежнему длинная.")},
		cats: []string{"gorod"}, tags: []string{"еда"}, daysAgo: 12, status: "published", byAuthor: true, popularity: 0.7},
	{typ: "event", title: "Фестиваль уличной музыки", slug: "festival-ulichnoy-muzyki",
		data: map[string]any{"venue": "Площадь у вокзала", "price": "Бесплатно", "body": "Двадцать коллективов, три сцены и ночной концерт.", "starts_at": "", "ends_at": ""},
		cats: []string{"kultura"}, tags: []string{"фестиваль", "музыка"}, daysAgo: 2, status: "published", cover: true, popularity: 0.8},
	{typ: "event", title: "Лекция «Архитектура модерна»", slug: "lekciya-modern",
		data: map[string]any{"venue": "Библиотека им. Маяковского", "price": "300 ₽", "body": "Как модерн изменил облик города за двадцать лет.", "starts_at": "", "ends_at": ""},
		cats: []string{"kultura"}, tags: []string{"лекции", "архитектура"}, daysAgo: 6, status: "published", popularity: 0.3},
	{typ: "article", title: "Почему в спальных районах не хватает школ", slug: "nehvatka-shkol",
		data: map[string]any{"lead": "Новые кварталы строят быстрее, чем социальную инфраструктуру.", "body": body("Мы сравнили планы застройки с реальными сроками ввода школ."), "reading_time": "11"},
		cats: []string{"gorod"}, tags: []string{"образование", "застройка"}, daysAgo: 15, status: "published", byAuthor: true, cover: true, popularity: 0.65},
	{typ: "news", title: "Футбольный клуб вышел в полуфинал кубка", slug: "polufinal-kubka",
		data: map[string]any{"lead": "Решающий гол забит на последней минуте.", "body": body("Следующий матч пройдёт на домашнем стадионе.")},
		cats: []string{"sport"}, tags: []string{"футбол"}, daysAgo: 20, status: "published", popularity: 0.55},
	{typ: "article", title: "Трамваи возвращаются: план развития сети до 2030 года", slug: "tramvai-2030",
		data: map[string]any{"lead": "Город обещает восстановить три маршрута и закупить 60 вагонов.", "body": body("Разбираем, какие районы получат трамвай первыми."), "reading_time": "9"},
		cats: []string{"transport"}, tags: []string{"транспорт", "трамвай"}, daysAgo: 25, status: "archived", cover: true, popularity: 0.35},
	{typ: "article", title: "Черновик: история одного дома", slug: "istoriya-doma",
		data: map[string]any{"lead": "Материал в работе.", "body": "Текст пишется."},
		cats: []string{"gorod"}, status: "draft", byAuthor: true},
	{typ: "news", title: "На модерации: новый график мусоровозов", slug: "grafik-musorovozov",
		data: map[string]any{"lead": "Вывоз будет ночным.", "body": "Ждём подтверждения от подрядчика."},
		cats: []string{"gorod"}, tags: []string{"жкх"}, status: "review", byAuthor: true},
	{typ: "page", title: "О редакции", slug: "about",
		data:    map[string]any{"body": "«Вестник» — городское онлайн-издание.\n\nМы пишем о том, что меняется рядом с вами: улицы, транспорт, культура.\n\n## Контакты\n\nПочта редакции: [redakciya@vestnik.local](mailto:redakciya@vestnik.local)", "show_in_menu": "on", "menu_order": "10"},
		daysAgo: 30, status: "published", popularity: 0.1},
	{typ: "page", title: "Правила комментирования", slug: "rules",
		data:    map[string]any{"body": "Будьте вежливы.", "show_in_menu": "", "menu_order": "90"},
		daysAgo: 30, status: "published"},
}

var categories = []struct{ slug, name, parent string }{
	{"gorod", "Город", ""}, {"transport", "Транспорт", "gorod"}, {"kultura", "Культура", ""},
	{"teatr", "Театр", "kultura"}, {"sport", "Спорт", ""},
}

// Run заполняет БД, если в ней ещё нет пользователей. adminEmail/adminPass — первый администратор.
func Run(ctx context.Context, a *app.App, adminEmail, adminPass string, log *slog.Logger) error {
	n, err := store.NewUserRepo(a.DB).Count(ctx)
	if err != nil || n > 0 {
		return err
	}
	if adminEmail == "" {
		adminEmail = "admin@vestnik.local"
	}
	if adminPass == "" {
		adminPass = DemoPass
		log.Warn("VESTNIK_ADMIN_PASSWORD не задан — используется демо-пароль, смените его", "email", adminEmail)
	}

	// Демо-данные «растянуты» на 30 дней назад: подменяем часы ядра, медиа и журнала на время заполнения.
	now := time.Now().UTC().Truncate(time.Minute)
	clock := now.AddDate(0, 0, -31)
	tick := func() time.Time { clock = clock.Add(time.Minute); return clock }
	oldStoreNow, oldContentNow, oldMediaNow := store.Now, a.Content.Now, a.Media.Now
	store.Now = func() string { return clock.Format(store.TimeFormat) }
	a.Content.Now, a.Media.Now = tick, tick
	defer func() { store.Now, a.Content.Now, a.Media.Now = oldStoreNow, oldContentNow, oldMediaNow }()

	users := map[string]access.Actor{}
	for _, u := range []struct{ email, name, role, pass string }{
		{adminEmail, "Администратор", "admin", adminPass},
		{EditorEmail, "Мария Редактор", "editor", DemoPass},
		{AuthorEmail, "Иван Корреспондент", "author", DemoPass},
	} {
		created, err := a.Auth.CreateUser(ctx, access.Actor{}, auth.UserInput{Email: u.email, Name: u.name, Role: u.role, Password: u.pass})
		if err != nil {
			return fmt.Errorf("seed user %s: %w", u.email, err)
		}
		users[u.role] = access.Actor{ID: created.ID, Name: created.Name, Role: created.Role}
	}
	editor, author := users["editor"], users["author"]

	catIDs := map[string]int64{}
	for _, c := range categories {
		t := &content.Term{Vocabulary: content.VocabCategory, Name: c.name, Slug: c.slug, ParentID: catIDs[c.parent]}
		if err := a.Taxonomy.Create(ctx, editor, t); err != nil {
			return fmt.Errorf("seed term %s: %w", c.slug, err)
		}
		catIDs[c.slug] = t.ID
	}

	rng := rand.New(rand.NewPCG(2026, 10))
	var published []struct {
		id         int64
		since      time.Time
		popularity float64
	}
	for i, it := range items {
		actor := editor
		if it.byAuthor {
			actor = author
		}
		if it.daysAgo > 0 || it.status == "published" {
			clock = now.AddDate(0, 0, -it.daysAgo).Add(-3 * time.Hour)
		} else {
			clock = now.Add(-2 * time.Hour)
		}
		data := map[string]any{}
		for k, v := range it.data {
			data[k] = v
		}
		if it.typ == "event" {
			start := now.AddDate(0, 0, 3+i).Truncate(24 * time.Hour).Add(16 * time.Hour)
			data["starts_at"] = start.Format(content.DateTimeLayout)
			data["ends_at"] = start.Add(4 * time.Hour).Format(content.DateTimeLayout)
		}
		if it.cover {
			m, err := a.Media.Upload(ctx, editor, it.slug+".jpg", bytes.NewReader(demoImage(i)))
			if err != nil {
				return fmt.Errorf("seed media: %w", err)
			}
			a.Media.SetAlt(ctx, editor, m.ID, it.title)
			data["cover"] = fmt.Sprint(m.ID)
		}
		in := content.Input{Title: it.title, Slug: it.slug, Data: data, Tags: it.tags}
		for _, c := range it.cats {
			in.CategoryIDs = append(in.CategoryIDs, catIDs[c])
		}
		e, err := a.Content.Create(ctx, actor, it.typ, in)
		if err != nil {
			return fmt.Errorf("seed entry %s: %w", it.slug, err)
		}
		steps := map[string][]string{"review": {"submit"}, "published": {"submit", "publish"}, "archived": {"submit", "publish", "archive"}}[it.status]
		for _, step := range steps {
			who := editor
			if step == "submit" {
				who = actor
			}
			clock = clock.Add(time.Duration(20+rng.IntN(160)) * time.Minute) // модерация занимает от 20 минут до 3 часов
			if _, err := a.Content.Transition(ctx, who, e.ID, step); err != nil {
				return fmt.Errorf("seed %s %s: %w", it.slug, step, err)
			}
		}
		if it.status == "published" || it.status == "archived" {
			published = append(published, struct {
				id         int64
				since      time.Time
				popularity float64
			}{e.ID, clock, it.popularity})
		}
	}
	// Правка опубликованного материала — чтобы были ревизии для демонстрации отката.
	if first, err := a.Content.GetBySlug(ctx, "article", items[0].slug); err == nil {
		clock = now.Add(-time.Hour)
		data := map[string]any{}
		for k, v := range first.Data {
			data[k] = v
		}
		data["lead"] = "Пешеходная зона станет вдвое шире, парковки уйдут под землю, а деревья сохранят."
		in := content.Input{Title: first.Title, Slug: first.Slug, Data: data}
		for _, c := range first.Categories {
			in.CategoryIDs = append(in.CategoryIDs, c.ID)
		}
		for _, t := range first.Tags {
			in.Tags = append(in.Tags, t.Name)
		}
		a.Content.Update(ctx, editor, first.ID, in)
	}

	// Просмотры: затухающий интерес после публикации, дневной шум, уникальные посетители.
	tx, err := a.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	views := 0
	for _, p := range published {
		for day := p.since; day.Before(now); day = day.Add(24 * time.Hour) {
			age := day.Sub(p.since).Hours() / 24
			count := int(p.popularity * 120 * math.Exp(-age/6) * (0.7 + 0.6*rng.Float64()))
			for v := 0; v < count; v++ {
				at := day.Add(time.Duration(rng.IntN(int(min(24*time.Hour, now.Sub(day)).Seconds())+1)) * time.Second)
				visitor := fmt.Sprintf("seed-%d-%d", at.YearDay(), rng.IntN(max(count*3/4, 1)))
				if _, err := tx.ExecContext(ctx, `INSERT INTO page_views (entry_id, visitor, referrer_host, created_at) VALUES (?, ?, ?, ?)`,
					p.id, visitor, []string{"", "", "yandex.ru", "vk.com", "t.me"}[rng.IntN(5)], at.Format(store.TimeFormat)); err != nil {
					return err
				}
				views++
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if _, err := a.Stats.Rollup(ctx); err != nil {
		return err
	}
	log.Info("seed: демо-данные созданы", "entries", len(items), "views", views,
		"admin", adminEmail, "editor", EditorEmail, "author", AuthorEmail)
	return nil
}

// demoImage рисует обложку: градиент и «солнце», цвета зависят от номера.
func demoImage(n int) []byte {
	const w, h = 1600, 900
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	hues := [][3]float64{{30, 60, 110}, {120, 40, 60}, {20, 90, 80}, {90, 70, 20}, {60, 40, 110}, {20, 70, 120}}
	c := hues[n%len(hues)]
	cx, cy, r := float64(w)*(0.3+0.08*float64(n%5)), float64(h)*0.4, 140.0+float64(n%4)*30
	for y := 0; y < h; y++ {
		t := float64(y) / h
		for x := 0; x < w; x++ {
			col := color.RGBA{uint8(c[0] + 120*t), uint8(c[1] + 100*t), uint8(c[2] + 60*(1-t)), 255}
			if dx, dy := float64(x)-cx, float64(y)-cy; dx*dx+dy*dy < r*r {
				col = color.RGBA{250, 220, 140, 255}
			}
			if y > h*2/3 && (x/80+n)%3 == 0 && y > h-(x%240)-140 { // силуэт «домов»
				col = color.RGBA{25, 25, 35, 255}
			}
			img.SetRGBA(x, y, col)
		}
	}
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 82})
	return buf.Bytes()
}
