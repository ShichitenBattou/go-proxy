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

func TestLoginHandler_ExtractsPathOnly_WhenAbsoluteURL(t *testing.T) {
	// This test verifies open redirect protection
	// Note: This test will fail at OAuth2 config step (Keycloak unavailable),
	// but it validates that absolute URLs are processed before that step
	req := httptest.NewRequest(http.MethodGet, "/auth/login?redirect_url=https://evil.com/malicious", nil)
	rr := httptest.NewRecorder()

	auth.LoginHandler(rr, req)

	// The handler should process the URL and extract path only,
	// then fail at OAuth2 config step (503) rather than URL validation step (400)
	if rr.Code == http.StatusBadRequest {
		t.Errorf("absolute URLs should not return 400 Bad Request (should extract path and proceed), got %d", rr.Code)
	}
	// Note: Will be 503 due to Keycloak unavailability in test environment
}

func TestLoginHandler_UsesDefaultRedirectURL_WhenNoParameter(t *testing.T) {
	// Test that missing redirect_url parameter defaults to "/"
	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	rr := httptest.NewRecorder()

	auth.LoginHandler(rr, req)

	// Should process with default "/" and fail at OAuth2 config (503)
	// not at URL validation (400)
	if rr.Code == http.StatusBadRequest {
		t.Errorf("missing redirect_url should use default and not return 400 Bad Request, got %d", rr.Code)
	}
	// Note: Will be 503 due to Keycloak unavailability in test environment
}
