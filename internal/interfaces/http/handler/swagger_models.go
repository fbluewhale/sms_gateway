package handler

import "time"

// SwaggerSMSRequest documents the public SMS request body. Runtime handlers use
// the application DTO with the same JSON contract.
type SwaggerSMSRequest struct {
	Line    string `json:"line" example:"express"`
	Dest    string `json:"dest" example:"989121234567"`
	Channel string `json:"channel" example:"main"`
	Message string `json:"message" example:"Your verification code is 123456"`
}

type SwaggerSMSResponse struct {
	Accepted         bool    `json:"accepted"`
	MessageID        string  `json:"message_id"`
	Cost             float64 `json:"cost"`
	RemainingBalance float64 `json:"remaining_balance"`
	Channel          string  `json:"channel"`
	Line             string  `json:"line"`
	Dest             string  `json:"dest"`
	Message          string  `json:"message"`
}

type SwaggerErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

type SwaggerDeliveryReport struct {
	MessageID   string     `json:"message_id"`
	WalletID    int64      `json:"wallet_id"`
	Destination string     `json:"destination"`
	Message     string     `json:"message"`
	Line        string     `json:"line"`
	Channel     string     `json:"channel"`
	Status      string     `json:"status"`
	Attempts    int        `json:"attempts"`
	LastError   string     `json:"last_error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
}
