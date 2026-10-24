// Package app собирает зависимости и HTTP-сервер.
package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ykkssyaa/vestnik-cms/internal/access"
	"github.com/ykkssyaa/vestnik-cms/internal/auth"
	"github.com/ykkssyaa/vestnik-cms/internal/config"
	"github.com/ykkssyaa/vestnik-cms/internal/content"
	"github.com/ykkssyaa/vestnik-cms/internal/media"
	"github.com/ykkssyaa/vestnik-cms/internal/render"
	"github.com/ykkssyaa/vestnik-cms/internal/store"
	"github.com/ykkssyaa/vestnik-cms/internal/web"
	"github.com/ykkssyaa/vestnik-cms/internal/web/admin"
	"github.com/ykkssyaa/vestnik-cms/internal/web/api"
	"github.com/ykkssyaa/vestnik-cms/internal/web/site"
)

type App struct {
	Cfg      config.Config
	DB       *sql.DB
	Log      *slog.Logger
	Types    *content.Registry
	Policy   *access.Policy
	Content  *content.Service
	Taxonomy *content.Taxonomy
	Media    *media.Service
	Auth     *auth.Service
	Render   *render.Engine
}

func New(cfg config.Config, log *slog.Logger) (*App, error) {
	db, err := store.Open(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("db: %w", err)
	}
	a, err := build(cfg, log, db)
	if err != nil {
		db.Close()
		return nil, err
	}
	return a, nil
}

func build(cfg config.Config, log *slog.Logger, db *sql.DB) (*App, error) {
	types, err := content.LoadRegistry(cfg.ContentTypesDir)
	if err != nil {
		return nil, err
	}
	policy, err := access.LoadPolicy(cfg.RolesFile)
	if err != nil {
		return nil, err
	}
	a := &App{Cfg: cfg, DB: db, Log: log, Types: types, Policy: policy}
	a.Auth = auth.NewService(db, policy)
	a.Content = content.NewService(db, types, policy)
	a.Content.Audit = a.Auth
	a.Taxonomy = &content.Taxonomy{DB: db, Auth: policy, Audit: a.Auth}
	a.Media = media.NewService(db, cfg.UploadsDir, policy)
	lookup := func(ctx context.Context, id int64) *store.Media {
		m, err := a.Media.Get(ctx, id)
		if err != nil {
			return nil
		}
		return m
	}
	if a.Render, err = render.New(cfg.ThemesDir, web.Funcs(types, lookup)); err != nil {
		return nil, fmt.Errorf("themes: %w", err)
	}

	synced, err := a.Content.Sync(context.Background())
	if err != nil {
		return nil, fmt.Errorf("sync content types: %w", err)
	}
	for _, r := range synced {
		log.Info("content type migrated", "type", r.Type, "from", r.From, "to", r.To, "entries", r.Affected)
	}
	return a, nil
}

func (a *App) Close() error { return a.DB.Close() }

// TestConfig — конфигурация для тестов: БД в памяти, каталоги проекта относительно root.
func TestConfig(root string) config.Config {
	return config.Config{
		DBPath: ":memory:", Theme: "classic", BaseURL: "http://test.local",
		ContentTypesDir: root + "/content_types", RolesFile: root + "/config/roles.json", ThemesDir: root + "/themes",
	}
}

// Handler возвращает корневой обработчик со всеми маршрутами.
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := a.DB.PingContext(r.Context()); err != nil {
			http.Error(w, "db down", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprintln(w, "ok")
	})
	mux.Handle("GET /static/{theme}/{file...}", a.Render.Static())
	// Медиа лежат как ГГГГ/ММ/имя: шаблон из трёх сегментов не пересекается с /{префикс}/{slug} сайта.
	mux.Handle("GET /uploads/{y}/{m}/{file}", http.StripPrefix("/uploads/", noDirListing(http.FileServer(http.Dir(a.Cfg.UploadsDir)))))

	settings := store.NewSettingsRepo(a.DB)
	lookup := func(ctx context.Context, id int64) *store.Media {
		if m, err := a.Media.Get(ctx, id); err == nil {
			return m
		}
		return nil
	}
	pub := &site.Handler{Content: a.Content, Taxonomy: a.Taxonomy, Render: a.Render, Settings: settings, Media: lookup,
		BaseURL: a.Cfg.BaseURL, DefaultTheme: a.Cfg.Theme, SiteName: "Вестник", Log: a.Log}
	pub.Routes(mux)
	(&api.Handler{Content: a.Content, Taxonomy: a.Taxonomy, Media: lookup, BaseURL: a.Cfg.BaseURL, Log: a.Log}).Routes(mux)

	adm := &admin.Handler{Content: a.Content, Taxonomy: a.Taxonomy, Media: a.Media, Auth: a.Auth, Policy: a.Policy,
		Render: a.Render, Settings: settings, ActiveTheme: pub.Theme, Log: a.Log,
		SecureCookie: strings.HasPrefix(a.Cfg.BaseURL, "https://")}
	adm.Routes(mux)
	return a.securityHeaders(a.logRequests(mux))
}

// noDirListing запрещает просмотр каталогов загрузок.
func noDirListing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") || r.URL.Path == "" {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

func (a *App) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		a.Log.Debug("request", "method", r.Method, "path", r.URL.Path)
	})
}
