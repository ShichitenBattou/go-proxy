package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"bff/auth"
	"bff/redis"
	"bff/setup"

	"github.com/google/uuid"
)

func TestMeHandler_ReturnsUserInfo_WhenSessionExists(t *testing.T) {
	cfg := setup.GetConfig()

	// Create test session
	sessionID := uuid.New().String()
	sessionData := redis.SessionData{
		UserID:       "user-123",
		Email:        "test@example.com",
		Name:         "Test User",
		AccessToken:  "access-token-abc",
		RefreshToken: "refresh-token-xyz",
	}

	if err := redis.SetSession(sessionID, sessionData); err != nil {
		t.Fatalf("Failed to set session: %v", err)
	}
	defer redis.DeleteSession(sessionID)

	// Create request with session cookie
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.AddCookie(&http.Cookie{
		Name:  cfg.SessionCookieName,
		Value: sessionID,
	})
	rr := httptest.NewRecorder()

	auth.MeHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response auth.UserInfoResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Sub != sessionData.UserID {
		t.Errorf("expected sub '%s', got '%s'", sessionData.UserID, response.Sub)
	}
	if response.Email != sessionData.Email {
		t.Errorf("expected email '%s', got '%s'", sessionData.Email, response.Email)
	}
	if response.Name != sessionData.Name {
		t.Errorf("expected name '%s', got '%s'", sessionData.Name, response.Name)
	}

	// Ensure tokens are NOT included in response
	responseBody := rr.Body.String()
	if len(responseBody) > 0 && (len(sessionData.AccessToken) > 0 || len(sessionData.RefreshToken) > 0) {
		// Parse as generic map to check for token fields
		var rawResponse map[string]interface{}
		json.Unmarshal([]byte(responseBody), &rawResponse)
		if _, hasAccess := rawResponse["access_token"]; hasAccess {
			t.Error("Response should not include access_token")
		}
		if _, hasRefresh := rawResponse["refresh_token"]; hasRefresh {
			t.Error("Response should not include refresh_token")
		}
	}
}

func TestMeHandler_ReturnsUnauthorized_WhenNoSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	rr := httptest.NewRecorder()

	auth.MeHandler(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["error"] != "Unauthorized" {
		t.Errorf("expected error 'Unauthorized', got '%s'", response["error"])
	}
}

func TestMeHandler_ReturnsUnauthorized_WhenSessionNotInRedis(t *testing.T) {
	cfg := setup.GetConfig()

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.AddCookie(&http.Cookie{
		Name:  cfg.SessionCookieName,
		Value: "nonexistent-session-id",
	})
	rr := httptest.NewRecorder()

	auth.MeHandler(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}
