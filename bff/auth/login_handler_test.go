package auth_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bff/auth"
)

func TestLoginHandler_ReturnsServiceUnavailable_WhenKeycloakDown(t *testing.T) {
	// This test verifies error handling when Keycloak is not available
	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	rr := httptest.NewRecorder()

	auth.LoginHandler(rr, req)

	// When Keycloak is unavailable, should return 503 Service Unavailable
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d (Service Unavailable), got %d", http.StatusServiceUnavailable, rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Authentication service unavailable") {
		t.Errorf("expected error message about service unavailable, got: %s", body)
	}
}
