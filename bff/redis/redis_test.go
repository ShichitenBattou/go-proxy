package redis_test

import (
	"testing"

	"bff/redis"
)

func TestSetAndGetSession(t *testing.T) {
	sessionId := "test-session-" + t.Name()
	sessionData := redis.SessionData{
		UserID: "test-user-123",
		Email:  "test@example.com",
		Name:   "Test User",
	}

	if err := redis.SetSession(sessionId, sessionData); err != nil {
		t.Fatalf("SetSession failed: %v", err)
	}

	val, err := redis.GetSessionValue(sessionId)
	if err != nil {
		t.Fatalf("GetSessionValue failed: %v", err)
	}

	if val.UserID != sessionData.UserID {
		t.Errorf("expected UserID '%s', got '%s'", sessionData.UserID, val.UserID)
	}
	if val.Email != sessionData.Email {
		t.Errorf("expected Email '%s', got '%s'", sessionData.Email, val.Email)
	}
	if val.Name != sessionData.Name {
		t.Errorf("expected Name '%s', got '%s'", sessionData.Name, val.Name)
	}
}

func TestDeleteSession(t *testing.T) {
	sessionId := "test-session-" + t.Name()
	sessionData := redis.SessionData{
		UserID: "test-user-456",
		Email:  "delete@example.com",
		Name:   "Delete Test",
	}

	if err := redis.SetSession(sessionId, sessionData); err != nil {
		t.Fatalf("SetSession failed: %v", err)
	}

	if err := redis.DeleteSession(sessionId); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	_, err := redis.GetSessionValue(sessionId)
	if err == nil {
		t.Error("expected error after deleting session, got nil")
	}
}

func TestGetSessionNotFound(t *testing.T) {
	_, err := redis.GetSessionValue("definitely-does-not-exist-session-xyz")
	if err == nil {
		t.Error("expected error for nonexistent session, got nil")
	}
}
