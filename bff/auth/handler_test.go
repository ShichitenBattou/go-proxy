package auth_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bff/auth"
)

func TestLoginHandler_RedirectsToKeycloak(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	rr := httptest.NewRecorder()

	auth.LoginHandler(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected status %d, got %d", http.StatusSeeOther, rr.Code)
	}

	location := rr.Header().Get("Location")
	if !strings.Contains(location, "realms/myrealm/protocol/openid-connect/auth") {
		t.Errorf("expected redirect to Keycloak OIDC endpoint, got: %s", location)
	}
	if !strings.Contains(location, "client_id=bff") {
		t.Errorf("expected client_id=bff in redirect URL, got: %s", location)
	}
	if !strings.Contains(location, "response_type=id_token") {
		t.Errorf("expected response_type=id_token in redirect URL, got: %s", location)
	}
	if !strings.Contains(location, "redirect_uri=") {
		t.Errorf("expected redirect_uri in redirect URL, got: %s", location)
	}
}

func TestCallbackHandler_ReturnsCallbackEndpoint(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/auth/callback", nil)
	rr := httptest.NewRecorder()

	auth.CallbackHandler(rr, req)

	body := rr.Body.String()
	if body != "Callback endpoint" {
		t.Errorf("expected body 'Callback endpoint', got: %s", body)
	}
}
