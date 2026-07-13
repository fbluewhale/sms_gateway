package sms

import (
	smsDomain "sms_gateway/internal/domain/sms"
)

type SendSMSRequest struct {
	Line    string `json:"line" binding:"required,oneof=express normal"`
	Dest    string `json:"dest" binding:"required"`
	Channel string `json:"channel" binding:"required"`
	Message string `json:"message" binding:"required,max=1600"`
}

type SendSMSResponse struct {
	Accepted         bool    `json:"accepted"`
	MessageID        string  `json:"message_id"`
	Cost             float64 `json:"cost"`
	RemainingBalance float64 `json:"remaining_balance"`
	Channel          string  `json:"channel"`
	Line             string  `json:"line"`
	Dest             string  `json:"dest"`
	Message          string  `json:"message"`
}

func ToCommand(req SendSMSRequest) SendSMSCommand {
	return SendSMSCommand{
		Line:        smsDomain.LineType(req.Line),
		Dest:        smsDomain.Destination(req.Dest),
		ChannelName: req.Channel,
		Message:     req.Message,
	}
}

func ToResponse(req SendSMSRequest, result *SendSMSResult) SendSMSResponse {
	return SendSMSResponse{
		Accepted:         true,
		MessageID:        result.MessageID,
		Cost:             result.Cost,
		RemainingBalance: result.RemainingBal,
		Channel:          req.Channel,
		Line:             req.Line,
		Dest:             req.Dest,
		Message:          req.Message,
	}
}
