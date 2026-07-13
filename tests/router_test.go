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

func TestSMSReportRoutesRequireAPIKey(t *testing.T) {
	r := router.Setup(&handler.SMSHandler{}, "secret")
	for _, path := range []string{"/api/v1/wallets/1/sms", "/api/v1/sms/SMS-1"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("path %s status = %d", path, resp.Code)
		}
	}
}

func TestSwaggerUIIsPublic(t *testing.T) {
	r := router.Setup(&handler.SMSHandler{}, "secret")
	req := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
}
