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

	handler := proxy.NewHandler(backend.Listener.Addr().String())

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.AddCookie(&http.Cookie{Name: "Session-Id", Value: "dummy-session"})
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Set-Cookie") == "" {
		t.Error("expected Set-Cookie header, got none")
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
	if err := redis.SetSession(sessionId, "127.0.0.1"); err != nil {
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
