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

func TestProxyHandler_NoSessionRotation(t *testing.T) {
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

	// Verify no Set-Cookie header (session rotation removed)
	if rr.Header().Get("Set-Cookie") != "" {
		t.Error("expected no Set-Cookie header (session rotation removed), got:", rr.Header().Get("Set-Cookie"))
	}

	// Verify original session remains in Redis
	_, err := redis.GetSessionValue(sessionId)
	if err != nil {
		t.Error("expected session to remain in Redis after request, got error:", err)
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

func TestProxyHandler_ExistingSessionPersists(t *testing.T) {
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
	defer redis.DeleteSession(sessionId)

	handler := proxy.NewHandler(backend.Listener.Addr().String())

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.AddCookie(&http.Cookie{Name: "Session-Id", Value: sessionId})
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Verify no Set-Cookie header (session not rotated)
	if rr.Header().Get("Set-Cookie") != "" {
		t.Error("expected no Set-Cookie header, got:", rr.Header().Get("Set-Cookie"))
	}

	// Verify session still exists with same data
	retrievedData, err := redis.GetSessionValue(sessionId)
	if err != nil {
		t.Fatal("expected session to persist in Redis, got error:", err)
	}
	if retrievedData.UserID != sessionData.UserID {
		t.Errorf("expected UserID %s, got %s", sessionData.UserID, retrievedData.UserID)
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

func TestProxyHandler_ConcurrentRequests(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	sessionId := "test-concurrent-" + t.Name()
	sessionData := redis.SessionData{
		UserID: "concurrent-user",
		Email:  "concurrent@example.com",
		Name:   "Concurrent Test",
	}
	if err := redis.SetSession(sessionId, sessionData); err != nil {
		t.Fatalf("failed to set up test session: %v", err)
	}
	defer redis.DeleteSession(sessionId)

	handler := proxy.NewHandler(backend.Listener.Addr().String())

	// Run 10 concurrent requests
	const concurrency = 10
	errors := make(chan error, concurrency)
	done := make(chan bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(index int) {
			req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
			req.AddCookie(&http.Cookie{Name: "Session-Id", Value: sessionId})
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				errors <- nil // Use a marker for failed status check
			}
			done <- true
		}(i)
	}

	// Wait for all requests to complete
	for i := 0; i < concurrency; i++ {
		<-done
	}
	close(errors)

	// Check if all requests succeeded (no race condition)
	errorCount := 0
	for range errors {
		errorCount++
	}
	if errorCount > 0 {
		t.Errorf("expected all concurrent requests to succeed, got %d failures", errorCount)
	}

	// Verify session still exists after all concurrent requests
	_, err := redis.GetSessionValue(sessionId)
	if err != nil {
		t.Error("expected session to remain in Redis after concurrent requests, got error:", err)
	}
}

func TestProxyHandler_Reprovisioning_Success(t *testing.T) {
	provisionCalled := false
	var receivedAuthHeader string

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/users" {
			provisionCalled = true
			w.WriteHeader(http.StatusCreated)
			return
		}
		receivedAuthHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	sessionId := "test-reprovision-success-" + t.Name()
	sessionData := redis.SessionData{
		UserID:      "test-user",
		Email:       "reprovision@example.com",
		Name:        "Reprovision Test",
		AccessToken: "test-access-token",
		Provisioned: false, // 未プロビジョニング状態
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

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !provisionCalled {
		t.Error("expected POST /users to be called for re-provisioning, but it was not")
	}
	if receivedAuthHeader != "Bearer test-access-token" {
		t.Errorf("expected Authorization header 'Bearer test-access-token', got '%s'", receivedAuthHeader)
	}

	// セッションの Provisioned フラグが true に更新されていることを確認
	updated, err := redis.GetSessionValue(sessionId)
	if err != nil {
		t.Fatalf("failed to get updated session: %v", err)
	}
	if !updated.Provisioned {
		t.Error("expected session Provisioned flag to be updated to true after successful re-provisioning")
	}
}

func TestProxyHandler_Reprovisioning_Failure(t *testing.T) {
	provisionCalled := false

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/users" {
			provisionCalled = true
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	sessionId := "test-reprovision-failure-" + t.Name()
	sessionData := redis.SessionData{
		UserID:      "test-user",
		Email:       "reprovision@example.com",
		Name:        "Reprovision Test",
		AccessToken: "test-access-token",
		Provisioned: false,
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

	// プロビジョニング失敗でもリクエストは転送される（ソフトフェイル）
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 even when re-provisioning fails, got %d", rr.Code)
	}
	if !provisionCalled {
		t.Error("expected POST /users to be called for re-provisioning, but it was not")
	}

	// Provisioned フラグは false のまま
	updated, err := redis.GetSessionValue(sessionId)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}
	if updated.Provisioned {
		t.Error("expected session Provisioned flag to remain false after failed re-provisioning")
	}
}
