package migrations

import (
	"context"
	"database/sql"
	"github.com/pressly/goose/v3"
)

func init() { goose.AddMigrationContext(upSMSOutbox, downSMSOutbox) }

func upSMSOutbox(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		CREATE TABLE sms_outbox (
			id BIGSERIAL PRIMARY KEY, message_id VARCHAR(255) NOT NULL UNIQUE,
			routing_key VARCHAR(32) NOT NULL, payload JSONB NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(), published_at TIMESTAMP NULL
		);
		CREATE INDEX idx_sms_outbox_pending ON sms_outbox (id) WHERE published_at IS NULL;
		CREATE TABLE sms_deliveries (
			message_id VARCHAR(255) PRIMARY KEY, status VARCHAR(32) NOT NULL,
			wallet_id BIGINT NOT NULL DEFAULT 0, destination VARCHAR(64) NOT NULL DEFAULT '',
			message TEXT NOT NULL DEFAULT '', line VARCHAR(32) NOT NULL DEFAULT '',
			channel_name VARCHAR(255) NOT NULL DEFAULT '',
			attempts INTEGER NOT NULL DEFAULT 0, last_error TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL DEFAULT NOW(), updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			delivered_at TIMESTAMP NULL
		);`)
	return err
}

func downSMSOutbox(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS sms_deliveries; DROP TABLE IF EXISTS sms_outbox;`)
	return err
}
