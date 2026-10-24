// Команда vestnik — сервер CMS и служебные подкоманды.
//
//	vestnik serve    — запустить сервер (по умолчанию)
//	vestnik migrate  — только применить миграции
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ykkssyaa/vestnik-cms/internal/app"
	"github.com/ykkssyaa/vestnik-cms/internal/config"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	if err := run(cmd, os.Args[2:], log); err != nil {
		log.Error("fatal", "cmd", cmd, "err", err)
		os.Exit(1)
	}
}

func run(cmd string, args []string, log *slog.Logger) error {
	cfg, err := config.Load(".env")
	if err != nil {
		return err
	}
	a, err := app.New(cfg, log)
	if err != nil {
		return err
	}
	defer a.Close()

	switch cmd {
	case "migrate":
		log.Info("migrations applied", "db", cfg.DBPath)
		return nil
	case "serve":
		return serve(a)
	default:
		return fmt.Errorf("неизвестная команда %q", cmd)
	}
}

func serve(a *app.App) error {
	srv := &http.Server{
		Addr:              a.Cfg.Addr,
		Handler:           a.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdown)
	}()
	a.Log.Info("listening", "addr", a.Cfg.Addr, "base_url", a.Cfg.BaseURL)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
