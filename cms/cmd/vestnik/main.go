// Команда vestnik — сервер CMS и служебные подкоманды.
//
//	vestnik serve    — запустить сервер (по умолчанию); при VESTNIK_SEED=1 заполняет пустую БД демо-данными
//	vestnik migrate  — только применить миграции
//	vestnik seed     — заполнить пустую БД демо-данными
//	vestnik rollup   — свернуть статистику просмотров за завершённые дни
//	vestnik adduser EMAIL ИМЯ РОЛЬ ПАРОЛЬ — создать пользователя
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

	"github.com/ykkssyaa/vestnik-cms/internal/access"
	"github.com/ykkssyaa/vestnik-cms/internal/app"
	"github.com/ykkssyaa/vestnik-cms/internal/auth"
	"github.com/ykkssyaa/vestnik-cms/internal/config"
	"github.com/ykkssyaa/vestnik-cms/internal/seed"
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
	ctx := context.Background()

	switch cmd {
	case "migrate":
		log.Info("migrations applied", "db", cfg.DBPath)
		return nil
	case "seed":
		return seed.Run(ctx, a, cfg.AdminEmail, cfg.AdminPassword, log)
	case "rollup":
		days, err := a.Stats.Rollup(ctx)
		log.Info("rollup", "days", days)
		return err
	case "serve":
		if cfg.Seed {
			if err := seed.Run(ctx, a, cfg.AdminEmail, cfg.AdminPassword, log); err != nil {
				return fmt.Errorf("seed: %w", err)
			}
		}
		return serve(a)
	case "adduser":
		if len(args) != 4 {
			return fmt.Errorf("использование: vestnik adduser EMAIL ИМЯ РОЛЬ ПАРОЛЬ")
		}
		u, err := a.Auth.CreateUser(ctx, access.Actor{},
			auth.UserInput{Email: args[0], Name: args[1], Role: args[2], Password: args[3]})
		if err != nil {
			return err
		}
		log.Info("user created", "id", u.ID, "email", u.Email, "role", u.Role)
		return nil
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
	go a.RunRollup(ctx, time.Hour)
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
