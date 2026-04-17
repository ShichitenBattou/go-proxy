package proxy_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"bff/proxy"
	"bff/redis"
)

func TestProxyHandler_PathRewriting(t *testing.T) {
	var receivedPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	handler := proxy.NewHandler(backend.Listener.Addr().String())

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.AddCookie(&http.Cookie{Name: "Session-Id", Value: "dummy-session"})
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if receivedPath != "/users" {
		t.Errorf("expected path '/users', got '%s'", receivedPath)
	}
}

func TestProxyHandler_SetsSessionCookie(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	// Create an existing session first
	sessionId := "test-session-cookie-" + t.Name()
	sessionData := redis.SessionData{
		UserID: "test-user",
		Email:  "cookie@example.com",
		Name:   "Cookie Test",
	}
	if err := redis.SetSession(sessionId, sessionData); err != nil {
		t.Fatalf("failed to set up test session: %v", err)
	}
	defer redis.DeleteSession(sessionId)

	handler := proxy.NewHandler(backend.Listener.Addr().String())

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.AddCookie(&http.Cookie{Name: "Session-Id", Value: sessionId})
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Set-Cookie") == "" {
		t.Error("expected Set-Cookie header for session rotation, got none")
	}
}

func TestProxyHandler_SetsRequestIdHeader(t *testing.T) {
	var receivedRequestId string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedRequestId = r.Header.Get("Request-Id")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	handler := proxy.NewHandler(backend.Listener.Addr().String())

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.AddCookie(&http.Cookie{Name: "Session-Id", Value: "dummy-session"})
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if receivedRequestId == "" {
		t.Error("expected Request-Id header to be forwarded to backend, got empty")
	}
}

func TestProxyHandler_ExistingSessionStored(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	sessionId := "test-proxy-session-" + t.Name()
	sessionData := redis.SessionData{
		UserID: "test-user",
		Email:  "test@example.com",
		Name:   "Test User",
	}
	if err := redis.SetSession(sessionId, sessionData); err != nil {
		t.Fatalf("failed to set up test session: %v", err)
	}

	handler := proxy.NewHandler(backend.Listener.Addr().String())

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.AddCookie(&http.Cookie{Name: "Session-Id", Value: sessionId})
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Set-Cookie") == "" {
		t.Error("expected new Set-Cookie header after session rotation")
	}
}

func TestProxyHandler_SetsAuthorizationHeader(t *testing.T) {
	var receivedAuthHeader string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	sessionId := "test-auth-session-" + t.Name()
	sessionData := redis.SessionData{
		UserID:      "test-user",
		Email:       "auth@example.com",
		Name:        "Auth Test",
		AccessToken: "test-access-token-12345",
	}
	if err := redis.SetSession(sessionId, sessionData); err != nil {
		t.Fatalf("failed to set up test session: %v", err)
	}
	defer redis.DeleteSession(sessionId)

	handler := proxy.NewHandler(backend.Listener.Addr().String())

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.AddCookie(&http.Cookie{Name: "Session-Id", Value: sessionId})
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	expectedAuth := "Bearer test-access-token-12345"
	if receivedAuthHeader != expectedAuth {
		t.Errorf("expected Authorization header '%s', got '%s'", expectedAuth, receivedAuthHeader)
	}
}

func TestProxyHandler_NoAuthorizationHeaderWhenTokenEmpty(t *testing.T) {
	var receivedAuthHeader string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	sessionId := "test-empty-token-" + t.Name()
	sessionData := redis.SessionData{
		UserID:      "test-user",
		Email:       "notoken@example.com",
		Name:        "No Token Test",
		AccessToken: "", // Empty token
	}
	if err := redis.SetSession(sessionId, sessionData); err != nil {
		t.Fatalf("failed to set up test session: %v", err)
	}
	defer redis.DeleteSession(sessionId)

	handler := proxy.NewHandler(backend.Listener.Addr().String())

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.AddCookie(&http.Cookie{Name: "Session-Id", Value: sessionId})
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if receivedAuthHeader != "" {
		t.Errorf("expected no Authorization header, got '%s'", receivedAuthHeader)
	}
}

func TestProxyHandler_NoAuthorizationHeaderWhenNoSession(t *testing.T) {
	var receivedAuthHeader string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	handler := proxy.NewHandler(backend.Listener.Addr().String())

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.AddCookie(&http.Cookie{Name: "Session-Id", Value: "non-existent-session"})
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if receivedAuthHeader != "" {
		t.Errorf("expected no Authorization header when session not found, got '%s'", receivedAuthHeader)
	}
}
