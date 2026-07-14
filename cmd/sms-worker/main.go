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
	"sms_gateway/internal/infrastructure/reservation"
	smsInfra "sms_gateway/internal/infrastructure/sms"
)

func main() {
	line := flag.String("line", "normal", "SMS line queue: express or normal")
	prefetch := flag.Int("prefetch", 10, "maximum unacknowledged messages")
	concurrency := flag.Int("concurrency", 5, "maximum concurrent provider calls")
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
	baseWalletRepo := persistence.NewPostgresWalletRepository(db)
	reservations, err := reservation.NewStore(cfg.RedisURL, baseWalletRepo)
	if err != nil {
		logger.Error("create Redis reservation store", "error", err)
		os.Exit(1)
	}
	defer reservations.Close()
	if err := reservations.Ping(ctx); err != nil {
		logger.Error("connect Redis", "error", err)
		os.Exit(1)
	}
	sender, err := smsInfra.NewDefaultRedisMockRoundRobinSender(ctx, cfg.RedisURL, logger, cfg.ProviderCircuitFailureThreshold, cfg.ProviderCircuitCooldown, cfg.ProviderTimeout)
	if err != nil {
		logger.Error("create SMS provider pool", "error", err)
		os.Exit(1)
	}
	defer sender.Close()
	worker, err := messaging.NewWorkerWithReservationAndTimeout(db, cfg.BrokerURL, *line, *prefetch, *concurrency, sender, reservations, cfg.ProviderTimeout)
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
