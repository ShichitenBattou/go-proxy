package auth

import (
	"bff/redis"
	"bff/setup"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// completeCallback のユニットテスト。
// Keycloak との通信を含まないため、Redis のみ必要（テスト実行時に起動済みであること）。

func testCfg() *setup.Config {
	return setup.GetConfig()
}

func testClaims() IDTokenClaims {
	return IDTokenClaims{
		Sub:   "test-sub-001",
		Email: "test@example.com",
		Name:  "Test User",
	}
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// provisionFn が失敗してもセッション Cookie がセットされることを検証する（ソフトフェイル）
func TestCompleteCallback_SoftFail_SessionCookieStillSet(t *testing.T) {
	failingProvision := func(_ context.Context, _, _ string) error {
		return fmt.Errorf("api is down")
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth/callback", nil)

	completeCallback(w, r, testCfg(), testClaims(), "raw-id-token", "api-token", "refresh-token", "/dashboard", failingProvision)

	result := w.Result()

	sessionCookie := findCookie(result.Cookies(), testCfg().SessionCookieName)
	if sessionCookie == nil {
		t.Fatal("session cookie must be set even when provisioning fails")
	}
	if sessionCookie.Value == "" {
		t.Error("session cookie value must not be empty")
	}
	if result.StatusCode != http.StatusFound {
		t.Errorf("expected redirect (302) even when provisioning fails, got %d", result.StatusCode)
	}
	if result.Header.Get("Location") != "/dashboard" {
		t.Errorf("expected redirect to /dashboard, got %s", result.Header.Get("Location"))
	}
}

// provisionFn が正しい引数（apiAccessToken, ProxyTarget）で呼ばれることを検証する
func TestCompleteCallback_Success_ProvisionCalledWithCorrectArgs(t *testing.T) {
	var gotToken, gotHost string
	provisionCalled := false

	trackingProvision := func(_ context.Context, token, host string) error {
		provisionCalled = true
		gotToken = token
		gotHost = host
		return nil
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth/callback", nil)

	completeCallback(w, r, testCfg(), testClaims(), "raw-id-token", "api-token", "refresh-token", "/", trackingProvision)

	if !provisionCalled {
		t.Fatal("provisionFn must be called")
	}
	if gotToken != "api-token" {
		t.Errorf("provisionFn received token %q, want %q", gotToken, "api-token")
	}
	if gotHost != testCfg().ProxyTarget {
		t.Errorf("provisionFn received host %q, want %q", gotHost, testCfg().ProxyTarget)
	}
}

// セッションに正しいユーザーデータが格納されることを検証する
func TestCompleteCallback_Success_SessionContainsUserData(t *testing.T) {
	noop := func(_ context.Context, _, _ string) error { return nil }

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth/callback", nil)
	claims := IDTokenClaims{Sub: "sub-abc", Email: "alice@example.com", Name: "Alice"}

	completeCallback(w, r, testCfg(), claims, "id-tok", "access-tok", "refresh-tok", "/", noop)

	result := w.Result()
	sessionCookie := findCookie(result.Cookies(), testCfg().SessionCookieName)
	if sessionCookie == nil {
		t.Fatal("session cookie not found")
	}

	data, err := redis.GetSessionValue(sessionCookie.Value)
	if err != nil {
		t.Fatalf("failed to get session from Redis: %v", err)
	}
	if data.UserID != "sub-abc" {
		t.Errorf("session UserID: want %q, got %q", "sub-abc", data.UserID)
	}
	if data.Email != "alice@example.com" {
		t.Errorf("session Email: want %q, got %q", "alice@example.com", data.Email)
	}
	if data.AccessToken != "access-tok" {
		t.Errorf("session AccessToken: want %q, got %q", "access-tok", data.AccessToken)
	}
}

// リダイレクト先が正しく設定されることを検証する
func TestCompleteCallback_Success_RedirectsToGivenURL(t *testing.T) {
	noop := func(_ context.Context, _, _ string) error { return nil }

	cases := []string{"/", "/dashboard", "/settings/profile"}
	for _, redirectURL := range cases {
		t.Run(redirectURL, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/auth/callback", nil)

			completeCallback(w, r, testCfg(), testClaims(), "id-tok", "access-tok", "refresh-tok", redirectURL, noop)

			result := w.Result()
			if result.StatusCode != http.StatusFound {
				t.Errorf("expected 302, got %d", result.StatusCode)
			}
			if result.Header.Get("Location") != redirectURL {
				t.Errorf("expected Location %q, got %q", redirectURL, result.Header.Get("Location"))
			}
		})
	}
}
