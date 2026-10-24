// Package app собирает зависимости и HTTP-сервер.
package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/ykkssyaa/vestnik-cms/internal/config"
	"github.com/ykkssyaa/vestnik-cms/internal/content"
	"github.com/ykkssyaa/vestnik-cms/internal/media"
	"github.com/ykkssyaa/vestnik-cms/internal/store"
)

type App struct {
	Cfg     config.Config
	DB      *sql.DB
	Log     *slog.Logger
	Types   *content.Registry
	Content *content.Service
	Media   *media.Service
}

func New(cfg config.Config, log *slog.Logger) (*App, error) {
	db, err := store.Open(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("db: %w", err)
	}
	types, err := content.LoadRegistry(cfg.ContentTypesDir)
	if err != nil {
		db.Close()
		return nil, err
	}
	a := &App{Cfg: cfg, DB: db, Log: log, Types: types}
	// Пока нет политики доступа (этап 3), сервис используется только для синхронизации схем.
	a.Content = content.NewService(db, types, nil)
	a.Media = media.NewService(db, cfg.UploadsDir, nil)
	synced, err := a.Content.Sync(context.Background())
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("sync content types: %w", err)
	}
	for _, r := range synced {
		log.Info("content type migrated", "type", r.Type, "from", r.From, "to", r.To, "entries", r.Affected)
	}
	return a, nil
}

func (a *App) Close() error { return a.DB.Close() }

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
	return a.logRequests(mux)
}

func (a *App) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		a.Log.Debug("request", "method", r.Method, "path", r.URL.Path)
	})
}
