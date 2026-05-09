package auth

import (
	"bff/redis"
	"bff/setup"
	"bff/utils"
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
)

// IDTokenClaims represents the claims from an OIDC ID Token
type IDTokenClaims struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
}

// NewCallbackHandler は ProvisionFunc を受け取り、OAuth2 コールバックハンドラーを返す。
// 本番では auth.NewCallbackHandler(auth.ProvisionUser) として使用する。
// テストでは mock の ProvisionFunc を渡してソフトフェイル挙動を検証できる。
func NewCallbackHandler(provisionFn ProvisionFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := setup.GetConfig()
		slog.Info("Received callback request", "url", r.URL.String(), "requestedHost", r.Host, "ip", r.RemoteAddr)
		slog.Debug("Request parameters", "query", r.URL.Query().Encode())

		// Validate and retrieve state token (CSRF protection)
		stateParam := r.URL.Query().Get("state")
		stateUUID, err := uuid.Parse(stateParam)
		if err != nil {
			slog.Error("Invalid state parameter", "state", stateParam, "error", err)
			http.Error(w, "Invalid state parameter", http.StatusForbidden)
			return
		}

		stateData, err := redis.GetStateValue(stateUUID)
		if err != nil {
			slog.Error("Failed to get state from Redis", "state", stateUUID, "error", err)
			http.Error(w, "Invalid state parameter", http.StatusForbidden)
			return
		}

		// Validate state token expiration (defense in depth with Redis TTL)
		if time.Since(stateData.CreatedAt) > cfg.StateTTL {
			slog.Error("State token expired", "state", stateUUID, "createdAt", stateData.CreatedAt, "age", time.Since(stateData.CreatedAt), "ttl", cfg.StateTTL)
			redis.DeleteState(stateUUID) // Clean up expired state
			http.Error(w, "State expired", http.StatusForbidden)
			return
		}

		// Delete state token (one-time use)
		if err := redis.DeleteState(stateUUID); err != nil {
			slog.Error("Failed to delete state from Redis", "error", err)
		}

		// Exchange authorization code for tokens
		ctx := oidc.ClientContext(context.Background(), utils.GetInternalHTTPClient())
		oauth2Config, err := getOAuth2Config()
		if err != nil {
			slog.Error("Failed to get OAuth2 config", "error", err)
			http.Error(w, "Authentication service unavailable", http.StatusServiceUnavailable)
			return
		}

		token, err := oauth2Config.Exchange(ctx, r.URL.Query().Get("code"))
		if err != nil {
			slog.Error("Failed to exchange code for token", "error", err)
			http.Error(w, "Authentication failed", http.StatusUnauthorized)
			return
		}
		slog.Info("Successfully exchanged code for tokens")

		// Exchange bff access token for api access token via RFC 8693 Token Exchange
		apiAccessToken, err := exchangeForAPIToken(ctx, token.AccessToken)
		if err != nil {
			slog.Error("Failed to exchange token for API access token", "error", err)
			http.Error(w, "Authentication failed", http.StatusUnauthorized)
			return
		}
		slog.Info("Successfully exchanged for API access token")

		// Extract and verify ID Token
		rawIDToken, ok := token.Extra("id_token").(string)
		if !ok {
			slog.Error("ID Token not found in token response")
			http.Error(w, "Authentication failed", http.StatusUnauthorized)
			return
		}

		provider, err := oidc.NewProvider(ctx, cfg.OIDCProviderURL)
		if err != nil {
			slog.Error("Failed to get OIDC provider", "error", err)
			http.Error(w, "Authentication failed", http.StatusInternalServerError)
			return
		}

		verifier := provider.Verifier(&oidc.Config{ClientID: cfg.OAuth2ClientID})
		idToken, err := verifier.Verify(ctx, rawIDToken)
		if err != nil {
			slog.Error("Failed to verify ID Token", "error", err)
			http.Error(w, "Invalid ID token", http.StatusUnauthorized)
			return
		}

		// Parse claims from ID Token
		var claims IDTokenClaims
		if err := idToken.Claims(&claims); err != nil {
			slog.Error("Failed to parse ID Token claims", "error", err)
			http.Error(w, "Authentication failed", http.StatusInternalServerError)
			return
		}

		slog.Info("ID Token verified", "sub", claims.Sub, "email", claims.Email)

		redirectURL := stateData.RedirectURL.String()
		if redirectURL == "" {
			redirectURL = "/"
		}

		completeCallback(w, r, cfg, callbackTokens{
			Claims:         claims,
			RawIDToken:     rawIDToken,
			APIAccessToken: apiAccessToken,
			RefreshToken:   token.RefreshToken,
			RedirectURL:    redirectURL,
		}, provisionFn)
	}
}

// callbackTokens はトークン交換完了後に completeCallback へ渡すデータをまとめた値オブジェクト。
// completeCallback のシグネチャを簡潔に保つために使用する。
type callbackTokens struct {
	Claims         IDTokenClaims
	RawIDToken     string
	APIAccessToken string
	RefreshToken   string
	RedirectURL    string
}

// completeCallback はトークン交換完了後のセッション生成・Cookie 設定・プロビジョニング・リダイレクトを行う。
// Keycloak との通信を含まないため、ユニットテストで直接検証できる。
func completeCallback(
	w http.ResponseWriter,
	r *http.Request,
	cfg *setup.Config,
	tokens callbackTokens,
	provisionFn ProvisionFunc,
) {
	claims := tokens.Claims
	// Create session data
	sessionData := redis.SessionData{
		UserID:       claims.Sub,
		Email:        claims.Email,
		Name:         claims.Name,
		IDToken:      tokens.RawIDToken,
		AccessToken:  tokens.APIAccessToken,
		RefreshToken: tokens.RefreshToken,
	}

	// Generate session ID and save to Redis
	sessionID := uuid.New().String()
	if err := redis.SetSession(sessionID, sessionData); err != nil {
		slog.Error("Failed to save session to Redis", "error", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	// Set session cookie with security attributes
	sameSite := http.SameSiteStrictMode
	switch cfg.SessionCookieSameSite {
	case "Lax":
		sameSite = http.SameSiteLaxMode
	case "None":
		sameSite = http.SameSiteNoneMode
	default:
		sameSite = http.SameSiteStrictMode
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cfg.SessionCookieName,
		Value:    sessionID,
		Path:     "/",
		Secure:   cfg.SessionCookieSecure,
		HttpOnly: cfg.SessionCookieHttpOnly,
		SameSite: sameSite,
	})

	slog.Info("Session created", "sessionId", sessionID, "userId", claims.Sub)

	// JIT プロビジョニング: API 側にユーザーレコードを作成する
	// エラー時はログのみ（ソフトフェイル）。API の一時停止でログイン自体を失敗させない。
	// 注意: プロビジョニング失敗時はセッションが作成されるが users テーブルにレコードが存在しない。
	// この場合、以降の認証済み API リクエストは get_current_user で 401 になる可能性がある。
	// ユーザーが再ログインすることでプロビジョニングが再試行される。
	if err := provisionFn(r.Context(), tokens.APIAccessToken, cfg.ProxyTarget); err != nil {
		slog.Warn("JIT provisioning failed; session created but user may be missing from DB",
			"sub", claims.Sub,
			"error", err,
		)
	}

	slog.Info("Redirecting to original URL", "url", tokens.RedirectURL)
	http.Redirect(w, r, tokens.RedirectURL, http.StatusFound)
}
