package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/discordbot/bot/internal/api"
	"github.com/discordbot/bot/internal/bot"
	"github.com/discordbot/bot/internal/config"
	"github.com/discordbot/bot/internal/database"
	"github.com/discordbot/bot/internal/store"
	"go.uber.org/zap"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal("config", zap.Error(err))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := database.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal("db connect", zap.Error(err))
	}
	defer db.Close()

	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		if err := db.Migrate(ctx, "migrations"); err != nil {
			log.Fatal("migrate", zap.Error(err))
		}
		log.Info("migrations applied")
		return
	}

	if err := db.Migrate(ctx, "migrations"); err != nil {
		log.Fatal("migrate", zap.Error(err))
	}

	st := store.New(db.Pool)

	b, err := bot.New(cfg.DiscordToken, cfg.DiscordAppID, st, log)
	if err != nil {
		log.Fatal("bot new", zap.Error(err))
	}
	if err := b.Start(ctx); err != nil {
		log.Fatal("bot start", zap.Error(err))
	}

	srv := &api.Server{
		Store:     st,
		Discord:   b.Session,
		Log:       log,
		JWTSecret: cfg.JWTSecret,
		Frontend:  cfg.FrontendURL,
		Port:      cfg.APIPort,
	}
	httpSrv := &http.Server{
		Addr:              ":" + cfg.APIPort,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("api listening", zap.String("port", cfg.APIPort))
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("api serve", zap.Error(err))
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Info("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	_ = b.Stop()
}
