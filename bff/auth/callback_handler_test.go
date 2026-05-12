package auth_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bff/auth"
)

func TestCallbackHandler_RejectsInvalidState(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?state=invalid-uuid&code=test", nil)
	rr := httptest.NewRecorder()

	auth.NewCallbackHandler(auth.ProvisionUser)(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Invalid state parameter") {
		t.Errorf("expected error message about invalid state, got: %s", body)
	}
}

func TestCallbackHandler_RejectsNonexistentState(t *testing.T) {
	// Use valid UUID format but nonexistent in Redis
	validUUID := "123e4567-e89b-12d3-a456-426614174000"
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?state="+validUUID+"&code=test", nil)
	rr := httptest.NewRecorder()

	auth.NewCallbackHandler(auth.ProvisionUser)(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Invalid state parameter") {
		t.Errorf("expected error message about invalid state, got: %s", body)
	}
}
