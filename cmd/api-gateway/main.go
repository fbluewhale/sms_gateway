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

	"sms_gateway/internal/application/admin"
	smsApp "sms_gateway/internal/application/sms"
	"sms_gateway/internal/config"
	"sms_gateway/internal/infrastructure/persistence"
	"sms_gateway/internal/infrastructure/reservation"
	"sms_gateway/internal/interfaces/http/handler"
	"sms_gateway/internal/interfaces/http/router"
)

func main() {
	if err := run(); err != nil {
		slog.Error("service stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := persistence.Connect(ctx, cfg.DB)
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get database handle: %w", err)
	}
	defer sqlDB.Close()

	if err := persistence.Migrate(db); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	channelRepo := persistence.NewPostgresChannelRepository(db)
	baseWalletRepo := persistence.NewPostgresWalletRepository(db)
	reservations, err := reservation.NewStore(cfg.RedisURL, baseWalletRepo)
	if err != nil {
		return fmt.Errorf("create Redis reservation store: %w", err)
	}
	defer reservations.Close()
	if err := reservations.Ping(ctx); err != nil {
		return fmt.Errorf("connect Redis: %w", err)
	}
	walletRepo := persistence.NewPostgresWalletRepositoryWithCache(db, reservations)
	smsCostRepo := persistence.NewPostgresSMSCostRepository(db)
	deliveryRepo := persistence.NewSMSDeliveryRepository(db)

	smsService := smsApp.NewServiceWithReservation(channelRepo, walletRepo, smsCostRepo, reservations, cfg.ExpressSLA, cfg.ExpressInFlight, cfg.NormalInFlight)
	adminService := admin.NewAdminService(walletRepo, channelRepo, deliveryRepo)

	h := handler.NewSMSHandler(smsService, adminService)
	r := router.Setup(h, cfg.AdminAPIKey)

	addr := fmt.Sprintf(":%s", cfg.Server.Port)
	server := &http.Server{Addr: addr, Handler: r, ReadHeaderTimeout: cfg.Server.ReadTimeout,
		ReadTimeout: cfg.Server.ReadTimeout, WriteTimeout: cfg.Server.WriteTimeout, IdleTimeout: cfg.Server.IdleTimeout}
	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe() }()
	logger.Info("SMS Gateway started", "address", addr)
	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown HTTP server: %w", err)
		}
		return nil
	}
}
