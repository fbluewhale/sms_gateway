package router

import (
	"crypto/subtle"
	"github.com/gin-gonic/gin"
	"net/http"

	"sms_gateway/internal/interfaces/http/handler"
)

func Setup(h *handler.SMSHandler, adminAPIKey string) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	api := r.Group("/api/v1")
	{
		api.POST("/sms", h.SendSMS)

		admin := api.Group("")
		admin.Use(requireAPIKey(adminAPIKey))
		admin.POST("/wallets", h.CreateWallet)
		admin.GET("/wallets/:id", h.GetWallet)
		admin.POST("/wallets/:id/topup", h.TopUpWallet)
		admin.GET("/wallets/:id/transactions", h.GetWalletTransactions)
		admin.GET("/wallets/:id/sms", h.ListSMSReports)
		admin.POST("/channels", h.CreateChannel)
		admin.GET("/channels", h.ListChannels)
		admin.GET("/sms/:message_id", h.GetSMSReport)
	}

	return r
}

func requireAPIKey(expected string) gin.HandlerFunc {
	return func(c *gin.Context) {
		provided := c.GetHeader("X-Admin-API-Key")
		if len(provided) != len(expected) || subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}
