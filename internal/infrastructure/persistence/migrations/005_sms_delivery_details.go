package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() { goose.AddMigrationContext(upSMSDeliveryDetails, downSMSDeliveryDetails) }

func upSMSDeliveryDetails(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		ALTER TABLE sms_deliveries ADD COLUMN IF NOT EXISTS wallet_id BIGINT NOT NULL DEFAULT 0;
		ALTER TABLE sms_deliveries ADD COLUMN IF NOT EXISTS destination VARCHAR(64) NOT NULL DEFAULT '';
		ALTER TABLE sms_deliveries ADD COLUMN IF NOT EXISTS message TEXT NOT NULL DEFAULT '';
		ALTER TABLE sms_deliveries ADD COLUMN IF NOT EXISTS line VARCHAR(32) NOT NULL DEFAULT '';
		ALTER TABLE sms_deliveries ADD COLUMN IF NOT EXISTS channel_name VARCHAR(255) NOT NULL DEFAULT '';
		CREATE INDEX IF NOT EXISTS idx_sms_deliveries_wallet_created ON sms_deliveries(wallet_id, created_at DESC);
	`)
	return err
}

func downSMSDeliveryDetails(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `DROP INDEX IF EXISTS idx_sms_deliveries_wallet_created;`)
	return err
}
