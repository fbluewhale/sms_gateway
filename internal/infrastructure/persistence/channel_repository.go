package persistence

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"sms_gateway/internal/domain/channel"
)

type PostgresChannelRepository struct {
	db *gorm.DB
}

func NewPostgresChannelRepository(db *gorm.DB) *PostgresChannelRepository {
	return &PostgresChannelRepository{db: db}
}

func (r *PostgresChannelRepository) GetByName(ctx context.Context, name string) (*channel.Channel, error) {
	var model ChannelModel
	if err := r.db.WithContext(ctx).Where("name = ?", name).First(&model).Error; err != nil {
		return nil, fmt.Errorf("channel not found: %w", err)
	}
	return toChannelModel(&model), nil
}

func (r *PostgresChannelRepository) Create(ctx context.Context, ch *channel.Channel) error {
	model := ChannelModel{
		Name:     ch.Name,
		WalletID: ch.WalletID,
		IsActive: ch.IsActive,
	}
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return fmt.Errorf("insert channel: %w", err)
	}
	result := toChannelModel(&model)
	ch.ID = result.ID
	ch.CreatedAt = result.CreatedAt
	ch.UpdatedAt = result.UpdatedAt
	return nil
}

func (r *PostgresChannelRepository) List(ctx context.Context) ([]channel.Channel, error) {
	var models []ChannelModel
	if err := r.db.WithContext(ctx).Order("id").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	channels := make([]channel.Channel, 0, len(models))
	for _, m := range models {
		channels = append(channels, *toChannelModel(&m))
	}
	return channels, nil
}
