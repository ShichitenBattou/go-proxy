package proxy_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"bff/proxy"
)

func TestPublicHandler_ForwardsPath(t *testing.T) {
	var receivedPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	handler := proxy.NewPublicHandler(backend.Listener.Addr().String())

	req := httptest.NewRequest(http.MethodGet, "/public/posts", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if receivedPath != "/public/posts" {
		t.Errorf("expected path '/public/posts', got '%s'", receivedPath)
	}
}

func TestPublicHandler_NoAuthorizationHeader(t *testing.T) {
	var receivedAuthHeader string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	handler := proxy.NewPublicHandler(backend.Listener.Addr().String())

	req := httptest.NewRequest(http.MethodGet, "/public/posts", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if receivedAuthHeader != "" {
		t.Errorf("expected no Authorization header, got '%s'", receivedAuthHeader)
	}
}

func TestPublicHandler_SetsRequestIdHeader(t *testing.T) {
	var receivedRequestId string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedRequestId = r.Header.Get("Request-Id")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	handler := proxy.NewPublicHandler(backend.Listener.Addr().String())

	req := httptest.NewRequest(http.MethodGet, "/public/posts", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if receivedRequestId == "" {
		t.Error("expected Request-Id header to be set, got empty")
	}
}

func TestPublicHandler_NoSessionCookieRequired(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	handler := proxy.NewPublicHandler(backend.Listener.Addr().String())

	// Request without any session cookie
	req := httptest.NewRequest(http.MethodGet, "/public/posts", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 without session cookie, got %d", rr.Code)
	}
}
