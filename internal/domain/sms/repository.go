package sms

import "context"

type SMSCost struct {
	ID       int64
	LineType LineType
	Cost     float64
}

type SMSCostRepository interface {
	GetByLineType(ctx context.Context, lineType LineType) (*SMSCost, error)
}
