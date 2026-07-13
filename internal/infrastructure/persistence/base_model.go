package persistence

import (
	"time"

	"sms_gateway/internal/domain/channel"
	"sms_gateway/internal/domain/sms"
	"sms_gateway/internal/domain/wallet"
)

type BaseModel struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type WalletModel struct {
	BaseModel
	Balance float64 `gorm:"type:decimal(15,4);not null;default:0"`
}

func (WalletModel) TableName() string {
	return "wallets"
}

type WalletTransactionModel struct {
	ID          int64     `gorm:"primaryKey" json:"id"`
	WalletID    int64     `gorm:"type:bigint;not null;index" json:"wallet_id"`
	Amount      float64   `gorm:"type:decimal(15,4);not null" json:"amount"`
	Type        string    `gorm:"type:varchar(50);not null" json:"type"`
	Description string    `gorm:"type:text;not null;default:''" json:"description"`
	ReferenceID string    `gorm:"type:varchar(255);not null;default:''" json:"reference_id"`
	CreatedAt   time.Time `json:"created_at"`
}

func (WalletTransactionModel) TableName() string {
	return "wallet_transactions"
}

type ChannelModel struct {
	BaseModel
	Name     string `gorm:"type:varchar(255);uniqueIndex;not null" json:"name"`
	WalletID int64  `gorm:"type:bigint;not null" json:"wallet_id"`
	IsActive bool   `gorm:"type:boolean;not null;default:true" json:"is_active"`
}

func (ChannelModel) TableName() string {
	return "channels"
}

type SMSCostModel struct {
	BaseModel
	LineType string  `gorm:"type:varchar(50);uniqueIndex;not null" json:"line_type"`
	Cost     float64 `gorm:"type:decimal(15,4);not null" json:"cost"`
}

type SMSOutboxModel struct {
	ID          int64      `gorm:"primaryKey"`
	MessageID   string     `gorm:"type:varchar(255);uniqueIndex;not null"`
	RoutingKey  string     `gorm:"type:varchar(32);not null;index"`
	Payload     []byte     `gorm:"type:jsonb;not null"`
	CreatedAt   time.Time  `gorm:"not null"`
	PublishedAt *time.Time `gorm:"index"`
}

func (SMSOutboxModel) TableName() string { return "sms_outbox" }

type SMSDeliveryModel struct {
	MessageID   string    `gorm:"type:varchar(255);primaryKey"`
	Status      string    `gorm:"type:varchar(32);not null"`
	Attempts    int       `gorm:"not null;default:0"`
	LastError   string    `gorm:"type:text;not null;default:''"`
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
	DeliveredAt *time.Time
}

func (SMSDeliveryModel) TableName() string { return "sms_deliveries" }

func (SMSCostModel) TableName() string {
	return "sms_costs"
}

func toWalletModel(m *WalletModel) *wallet.Wallet {
	return &wallet.Wallet{
		ID:        m.ID,
		Balance:   wallet.MustMoney(m.Balance),
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

func toWalletTransactionModel(m *WalletTransactionModel) wallet.WalletTransaction {
	return wallet.WalletTransaction{
		ID:          m.ID,
		WalletID:    m.WalletID,
		Amount:      m.Amount,
		Type:        wallet.TransactionType(m.Type),
		Description: m.Description,
		ReferenceID: m.ReferenceID,
		CreatedAt:   m.CreatedAt,
	}
}

func toChannelModel(m *ChannelModel) *channel.Channel {
	return &channel.Channel{
		ID:        m.ID,
		Name:      m.Name,
		WalletID:  m.WalletID,
		IsActive:  m.IsActive,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

func toSMSCostModel(m *SMSCostModel) *sms.SMSCost {
	return &sms.SMSCost{
		ID:       m.ID,
		LineType: sms.LineType(m.LineType),
		Cost:     m.Cost,
	}
}
