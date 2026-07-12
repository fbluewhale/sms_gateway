package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upSeedSMSCosts, downSeedSMSCosts)
}

func upSeedSMSCosts(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO sms_costs (line_type, cost)
		VALUES
			('express', 2.50),
			('normal', 1.00)
		ON CONFLICT (line_type) DO NOTHING;
	`)
	return err
}

func downSeedSMSCosts(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		DELETE FROM sms_costs WHERE line_type IN ('express', 'normal');
	`)
	return err
}
