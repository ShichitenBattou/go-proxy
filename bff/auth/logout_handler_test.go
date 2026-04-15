package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"bff/auth"
	"bff/redis"
	"bff/setup"

	"github.com/google/uuid"
)

func TestLogoutHandler_DeletesSessionAndRedirects(t *testing.T) {
	cfg := setup.GetConfig()

	// Create test session
	sessionID := uuid.New().String()
	sessionData := redis.SessionData{
		UserID:  "user-logout-123",
		Email:   "logout@example.com",
		Name:    "Logout User",
		IDToken: "test-id-token-abc",
	}

	if err := redis.SetSession(sessionID, sessionData); err != nil {
		t.Fatalf("Failed to set session: %v", err)
	}

	// Create request with session cookie
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.AddCookie(&http.Cookie{
		Name:  cfg.SessionCookieName,
		Value: sessionID,
	})
	rr := httptest.NewRecorder()

	auth.LogoutHandler(rr, req)

	// Check redirect status
	if rr.Code != http.StatusFound {
		t.Errorf("expected status %d, got %d", http.StatusFound, rr.Code)
	}

	// Check redirect happens (Keycloak or fallback)
	location := rr.Header().Get("Location")
	if location == "" {
		t.Error("expected redirect Location header, got none")
	}
	// In unit test environment (no Keycloak), expects fallback to https://auth.local/
	// In integration test environment (with Keycloak), would redirect to Keycloak logout endpoint

	// Check session is deleted from Redis (most important)
	_, err := redis.GetSessionValue(sessionID)
	if err == nil {
		t.Error("expected session to be deleted from Redis, but it still exists")
	}

	// Check Set-Cookie header for deletion
	cookies := rr.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == cfg.SessionCookieName {
			sessionCookie = c
			break
		}
	}

	if sessionCookie == nil {
		t.Error("expected Set-Cookie header for session deletion")
	} else if sessionCookie.MaxAge != -1 {
		t.Errorf("expected MaxAge=-1 for cookie deletion, got %d", sessionCookie.MaxAge)
	}
}

func TestLogoutHandler_RedirectsEvenWithoutSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	rr := httptest.NewRecorder()

	auth.LogoutHandler(rr, req)

	// Should redirect to post_logout_redirect_uri even without session
	if rr.Code != http.StatusFound {
		t.Errorf("expected status %d, got %d", http.StatusFound, rr.Code)
	}

	location := rr.Header().Get("Location")
	if location != "https://auth.local/" {
		t.Errorf("expected redirect to https://auth.local/, got: %s", location)
	}
}

func TestLogoutHandler_HandlesNonexistentSession(t *testing.T) {
	cfg := setup.GetConfig()

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.AddCookie(&http.Cookie{
		Name:  cfg.SessionCookieName,
		Value: "nonexistent-session-id",
	})
	rr := httptest.NewRecorder()

	auth.LogoutHandler(rr, req)

	// Should still redirect (Keycloak or fallback)
	if rr.Code != http.StatusFound {
		t.Errorf("expected status %d, got %d", http.StatusFound, rr.Code)
	}

	location := rr.Header().Get("Location")
	if location == "" {
		t.Error("expected redirect Location header, got none")
	}

	// Check cookie deletion header is set
	cookies := rr.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == cfg.SessionCookieName {
			sessionCookie = c
			break
		}
	}

	if sessionCookie == nil {
		t.Error("expected Set-Cookie header for session deletion")
	} else if sessionCookie.MaxAge != -1 {
		t.Errorf("expected MaxAge=-1 for cookie deletion, got %d", sessionCookie.MaxAge)
	}
}
