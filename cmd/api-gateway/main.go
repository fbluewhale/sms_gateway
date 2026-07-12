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
	smsInfra "sms_gateway/internal/infrastructure/sms"
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
	walletRepo := persistence.NewPostgresWalletRepository(db)
	smsCostRepo := persistence.NewPostgresSMSCostRepository(db)

	smsService := smsApp.NewService(channelRepo, walletRepo, smsCostRepo, smsInfra.NewMockSender(logger))
	adminService := admin.NewAdminService(walletRepo, channelRepo)

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
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
		defer cancel()
		if shutdownErr := smsService.Shutdown(shutdownCtx); shutdownErr != nil {
			return shutdownErr
		}
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
		if err := smsService.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return nil
	}
}
