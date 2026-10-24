// Package app собирает зависимости и HTTP-сервер.
package app

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/ykkssyaa/vestnik-cms/internal/config"
	"github.com/ykkssyaa/vestnik-cms/internal/store"
)

type App struct {
	Cfg config.Config
	DB  *sql.DB
	Log *slog.Logger
}

func New(cfg config.Config, log *slog.Logger) (*App, error) {
	db, err := store.Open(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("db: %w", err)
	}
	return &App{Cfg: cfg, DB: db, Log: log}, nil
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
