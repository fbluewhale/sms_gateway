package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upCreateTables, downCreateTables)
}

func upCreateTables(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS wallets (
			id BIGSERIAL PRIMARY KEY,
			balance DECIMAL(15,4) NOT NULL DEFAULT 0.0,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		);
	`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS channels (
			id BIGSERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL UNIQUE,
			wallet_id BIGINT NOT NULL REFERENCES wallets(id),
			is_active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		);
	`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS sms_costs (
			id BIGSERIAL PRIMARY KEY,
			line_type VARCHAR(50) NOT NULL UNIQUE,
			cost DECIMAL(15,4) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		);
	`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS wallet_transactions (
			id BIGSERIAL PRIMARY KEY,
			wallet_id BIGINT NOT NULL REFERENCES wallets(id),
			amount DECIMAL(15,4) NOT NULL,
			type VARCHAR(50) NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			reference_id VARCHAR(255) NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		);
	`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_wallet_transactions_wallet_id ON wallet_transactions(wallet_id);
	`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_channels_name ON channels(name);
	`)
	return err
}

func downCreateTables(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS wallet_transactions;
		DROP TABLE IF EXISTS sms_costs;
		DROP TABLE IF EXISTS channels;
		DROP TABLE IF EXISTS wallets;
	`)
	return err
}
