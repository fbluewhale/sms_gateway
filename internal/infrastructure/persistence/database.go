package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/pressly/goose/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"sms_gateway/internal/config"
	_ "sms_gateway/internal/infrastructure/persistence/migrations"
)

func Connect(ctx context.Context, cfg config.DatabaseConfig) (*gorm.DB, error) {
	dsn := cfg.DSN()

	var db *gorm.DB
	var err error

	for i := 0; i < 30; i++ {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger:                                   logger.Default.LogMode(logger.Silent),
			DisableForeignKeyConstraintWhenMigrating: true,
			NowFunc:                                  func() time.Time { return time.Now().UTC() },
		})
		if err == nil {
			sqlDB, dbErr := db.DB()
			if dbErr == nil {
				dbErr = sqlDB.PingContext(ctx)
			}
			if dbErr == nil {
				break
			}
			err = dbErr
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("connect database: %w", ctx.Err())
		case <-time.After(2 * time.Second):
		}
	}

	if err != nil {
		return nil, fmt.Errorf("connect database after retries: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get underlying sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	return db, nil
}

func Migrate(db *gorm.DB) error {
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get underlying sql.DB: %w", err)
	}

	if err := goose.Up(sqlDB, "."); err != nil {
		return fmt.Errorf("goose migration: %w", err)
	}

	return nil
}
