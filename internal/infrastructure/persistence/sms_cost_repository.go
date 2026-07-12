package persistence

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"sms_gateway/internal/domain/sms"
)

type PostgresSMSCostRepository struct {
	db *gorm.DB
}

func NewPostgresSMSCostRepository(db *gorm.DB) *PostgresSMSCostRepository {
	return &PostgresSMSCostRepository{db: db}
}

func (r *PostgresSMSCostRepository) GetByLineType(ctx context.Context, lineType sms.LineType) (*sms.SMSCost, error) {
	var model SMSCostModel
	if err := r.db.WithContext(ctx).Where("line_type = ?", lineType.String()).First(&model).Error; err != nil {
		return nil, fmt.Errorf("sms cost not found for line type '%s': %w", lineType, err)
	}
	return toSMSCostModel(&model), nil
}
