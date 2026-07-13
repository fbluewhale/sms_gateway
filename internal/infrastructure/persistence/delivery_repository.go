package persistence

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"sms_gateway/internal/domain/sms"
)

type SMSDeliveryRepository struct{ db *gorm.DB }

func NewSMSDeliveryRepository(db *gorm.DB) *SMSDeliveryRepository {
	return &SMSDeliveryRepository{db: db}
}

func (r *SMSDeliveryRepository) ListByWallet(ctx context.Context, walletID int64, limit int) ([]sms.DeliveryReport, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	var rows []SMSDeliveryModel
	if err := r.db.WithContext(ctx).Where("wallet_id = ?", walletID).Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list SMS delivery reports: %w", err)
	}
	result := make([]sms.DeliveryReport, 0, len(rows))
	for _, row := range rows {
		result = append(result, toDeliveryReport(row))
	}
	return result, nil
}

func (r *SMSDeliveryRepository) GetByMessageID(ctx context.Context, messageID string) (*sms.DeliveryReport, error) {
	var row SMSDeliveryModel
	if err := r.db.WithContext(ctx).First(&row, "message_id = ?", messageID).Error; err != nil {
		return nil, fmt.Errorf("get SMS delivery report: %w", err)
	}
	report := toDeliveryReport(row)
	return &report, nil
}

func toDeliveryReport(row SMSDeliveryModel) sms.DeliveryReport {
	return sms.DeliveryReport{MessageID: row.MessageID, WalletID: row.WalletID, Destination: sms.Destination(row.Destination), Message: row.Message, Line: sms.LineType(row.Line), ChannelName: row.ChannelName, Status: row.Status, Attempts: row.Attempts, LastError: row.LastError, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, DeliveredAt: row.DeliveredAt}
}
