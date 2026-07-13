package sms

import "time"

type LineType string

const (
	LineTypeExpress LineType = "express"
	LineTypeNormal  LineType = "normal"
)

func (lt LineType) IsValid() bool {
	return lt == LineTypeExpress || lt == LineTypeNormal
}

func (lt LineType) String() string {
	return string(lt)
}

type Destination string

func (d Destination) String() string {
	return string(d)
}

func (d Destination) IsValid() bool {
	return len(d) > 0
}

func IsMessageValid(message string) bool {
	return len([]rune(message)) > 0 && len([]rune(message)) <= 1600
}

type DeliveryReport struct {
	MessageID   string
	WalletID    int64
	Destination Destination
	Message     string
	Line        LineType
	ChannelName string
	Status      string
	Attempts    int
	LastError   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeliveredAt *time.Time
}
