package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"sms_gateway/internal/config"
	"sms_gateway/internal/infrastructure/messaging"
	"sms_gateway/internal/infrastructure/persistence"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}
	db, err := persistence.Connect(ctx, cfg.DB)
	if err != nil {
		slog.Error("connect database", "error", err)
		os.Exit(1)
	}
	dispatcher, err := messaging.NewDispatcher(db, cfg.BrokerURL)
	if err != nil {
		slog.Error("create dispatcher", "error", err)
		os.Exit(1)
	}
	defer dispatcher.Close()
	if err := dispatcher.Run(ctx); err != nil {
		slog.Error("run dispatcher", "error", err)
		os.Exit(1)
	}
}
