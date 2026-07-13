package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"sms_gateway/internal/config"
	"sms_gateway/internal/infrastructure/messaging"
	"sms_gateway/internal/infrastructure/persistence"
	smsInfra "sms_gateway/internal/infrastructure/sms"
)

func main() {
	line := flag.String("line", "normal", "SMS line queue: express or normal")
	prefetch := flag.Int("prefetch", 10, "maximum unacknowledged messages")
	flag.Parse()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}
	db, err := persistence.Connect(ctx, cfg.DB)
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	worker, err := messaging.NewWorker(db, cfg.BrokerURL, *line, *prefetch, smsInfra.NewMockSender(logger))
	if err != nil {
		logger.Error("create worker", "error", err)
		os.Exit(1)
	}
	defer worker.Close()
	if err := worker.Run(ctx); err != nil {
		logger.Error("run worker", "error", err)
		os.Exit(1)
	}
}
