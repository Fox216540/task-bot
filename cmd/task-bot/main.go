package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"task-bot/internal/bot"
	"task-bot/internal/config"
	"task-bot/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	repo, err := storage.NewSQLiteRepo(cfg.DBPath)
	if err != nil {
		log.Fatalf("db init error: %v", err)
	}
	defer repo.Close()

	api, err := bot.NewAPIClient(cfg.BotToken)
	if err != nil {
		log.Fatalf("telegram init error: %v", err)
	}
	svc := bot.NewService(repo, api, cfg.AllowedUserID)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := svc.RunReminderCycle(ctx); err != nil {
					log.Printf("reminder cycle error: %v", err)
				}
			}
		}
	}()

	if err := svc.RunPolling(ctx); err != nil && ctx.Err() == nil {
		log.Fatalf("polling error: %v", err)
	}
}
