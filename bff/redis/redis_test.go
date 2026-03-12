package redis_test

import (
	"testing"

	"bff/redis"
)

func TestSetAndGetSession(t *testing.T) {
	sessionId := "test-session-" + t.Name()

	if err := redis.SetSession(sessionId, "127.0.0.1"); err != nil {
		t.Fatalf("SetSession failed: %v", err)
	}

	val, err := redis.GetSessionValue(sessionId)
	if err != nil {
		t.Fatalf("GetSessionValue failed: %v", err)
	}

	if val != "127.0.0.1" {
		t.Errorf("expected '127.0.0.1', got '%s'", val)
	}
}

func TestDeleteSession(t *testing.T) {
	sessionId := "test-session-" + t.Name()

	if err := redis.SetSession(sessionId, "10.0.0.1"); err != nil {
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
