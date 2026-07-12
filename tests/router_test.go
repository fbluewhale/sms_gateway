package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"sms_gateway/internal/interfaces/http/handler"
	"sms_gateway/internal/interfaces/http/router"
)

func TestAdminRoutesRequireAPIKey(t *testing.T) {
	r := router.Setup(&handler.SMSHandler{}, "secret")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/channels", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", resp.Code)
	}
}
