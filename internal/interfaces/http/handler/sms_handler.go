package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"sms_gateway/internal/application/admin"
	appDTO "sms_gateway/internal/application/sms"
	"sms_gateway/internal/domain/wallet"
	"sms_gateway/internal/interfaces/http/dto"
)

type SMSHandler struct {
	smsService   *appDTO.Service
	adminService *admin.AdminService
}

func NewSMSHandler(smsService *appDTO.Service, adminService *admin.AdminService) *SMSHandler {
	return &SMSHandler{
		smsService:   smsService,
		adminService: adminService,
	}
}

func (h *SMSHandler) SendSMS(c *gin.Context) {
	var req appDTO.SendSMSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "validation failed",
			Details: err.Error(),
		})
		return
	}

	cmd := appDTO.ToCommand(req)
	result, err := h.smsService.Execute(c.Request.Context(), cmd)
	if err != nil {
		writeServiceError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, appDTO.ToResponse(req, result))
}

func (h *SMSHandler) CreateWallet(c *gin.Context) {
	var req admin.CreateWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	w, err := h.adminService.CreateWallet(c.Request.Context(), req.Balance)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, admin.ToWalletResponse(w))
}

func (h *SMSHandler) GetWallet(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	w, err := h.adminService.GetWallet(c.Request.Context(), id)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, admin.ToWalletResponse(w))
}

func (h *SMSHandler) TopUpWallet(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var req admin.TopUpWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	w, err := h.adminService.TopUpWallet(c.Request.Context(), id, req.Amount, req.ReferenceID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, admin.ToWalletResponse(w))
}

func (h *SMSHandler) CreateChannel(c *gin.Context) {
	var req admin.CreateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	ch, err := h.adminService.CreateChannel(c.Request.Context(), req.Name, req.WalletID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, admin.ToChannelResponse(*ch))
}

func (h *SMSHandler) ListChannels(c *gin.Context) {
	channels, err := h.adminService.ListChannels(c.Request.Context())
	if err != nil {
		writeServiceError(c, err)
		return
	}
	resp := make([]admin.ChannelResponse, 0, len(channels))
	for _, ch := range channels {
		resp = append(resp, admin.ToChannelResponse(ch))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *SMSHandler) GetWalletTransactions(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	txs, err := h.adminService.GetWalletTransactions(c.Request.Context(), id)
	if err != nil {
		writeServiceError(c, err)
		return
	}

	resp := make([]admin.TransactionResponse, 0, len(txs))
	for _, tx := range txs {
		resp = append(resp, admin.ToTransactionResponse(tx))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *SMSHandler) ListSMSReports(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	limit := 100
	if raw := c.Query("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	reports, err := h.adminService.ListSMSReports(c.Request.Context(), id, limit)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response := make([]admin.SMSDeliveryResponse, 0, len(reports))
	for _, report := range reports {
		response = append(response, admin.ToSMSDeliveryResponse(report))
	}
	c.JSON(http.StatusOK, response)
}

func (h *SMSHandler) GetSMSReport(c *gin.Context) {
	messageID := strings.TrimSpace(c.Param("message_id"))
	if messageID == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "message_id is required"})
		return
	}
	report, err := h.adminService.GetSMSReport(c.Request.Context(), messageID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, admin.ToSMSDeliveryResponse(*report))
}

func writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, admin.ErrInvalidInput), errors.Is(err, wallet.ErrInvalidAmount):
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid request"})
	case errors.Is(err, wallet.ErrInsufficientFunds):
		c.JSON(http.StatusUnprocessableEntity, dto.ErrorResponse{Error: "insufficient balance"})
	case errors.Is(err, appDTO.ErrInsufficientCredit):
		c.JSON(http.StatusUnprocessableEntity, dto.ErrorResponse{Error: "insufficient balance"})
	case errors.Is(err, appDTO.ErrLineOverloaded):
		c.Header("Retry-After", "1")
		c.JSON(http.StatusTooManyRequests, dto.ErrorResponse{Error: "SMS line is at capacity"})
	default:
		c.Error(err) //nolint:errcheck -- captured by request logger
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
	}
}
