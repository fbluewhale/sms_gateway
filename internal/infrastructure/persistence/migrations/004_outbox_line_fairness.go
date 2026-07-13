package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() { goose.AddMigrationContext(upOutboxLineFairness, downOutboxLineFairness) }

func upOutboxLineFairness(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		CREATE INDEX idx_sms_outbox_pending_line
		ON sms_outbox (routing_key, id)
		WHERE published_at IS NULL;
	`)
	return err
}

func downOutboxLineFairness(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `DROP INDEX IF EXISTS idx_sms_outbox_pending_line;`)
	return err
}
