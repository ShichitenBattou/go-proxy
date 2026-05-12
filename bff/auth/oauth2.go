package auth

import (
	"bff/setup"
	"bff/redis"
	"bff/utils"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

func getOAuth2Config() (*oauth2.Config, error) {
	cfg := setup.GetConfig()
	ctx := oidc.ClientContext(context.Background(), utils.GetInternalHTTPClient())
	provider, err := oidc.NewProvider(ctx, cfg.OIDCProviderURL)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			slog.Error("OIDC provider initialization timed out. Ensure Keycloak is running and accessible", "providerURL", cfg.OIDCProviderURL, "error", err)
		} else {
			slog.Error("Failed to initialize OIDC provider", "error", err)
		}
		return nil, err
	}

	oauth2Config := oauth2.Config{
		ClientID:     cfg.OAuth2ClientID,
		ClientSecret: cfg.OAuth2ClientSecret,
		RedirectURL:  cfg.OAuth2RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       cfg.OAuth2Scopes,
	}
	return &oauth2Config, nil
}

// exchangeForAPIToken exchanges a bff access token for an api access token
// using the RFC 8693 Token Exchange grant type.
func exchangeForAPIToken(ctx context.Context, bffAccessToken string) (string, error) {
	cfg := setup.GetConfig()
	tokenURL := cfg.OIDCProviderURL + "/protocol/openid-connect/token"

	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", bffAccessToken)
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:access_token")
	formData.Set("requested_token_type", "urn:ietf:params:oauth:token-type:access_token")
	formData.Set("audience", cfg.OAuth2TargetAudience)
	formData.Set("client_id", cfg.OAuth2ClientID)
	formData.Set("client_secret", cfg.OAuth2ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create token exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := utils.GetInternalHTTPClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token exchange response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("token exchange response missing access_token")
	}

	return tokenResp.AccessToken, nil
}

// refreshBFFToken は RefreshToken を使って Keycloak から新しい BFF トークンセット
//（AccessToken + RefreshToken）を取得する。
func refreshBFFToken(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
	ctx = oidc.ClientContext(ctx, utils.GetInternalHTTPClient())
	oauth2Config, err := getOAuth2Config()
	if err != nil {
		return nil, fmt.Errorf("failed to get OAuth2 config: %w", err)
	}
	ts := oauth2Config.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})
	token, err := ts.Token()
	if err != nil {
		return nil, fmt.Errorf("token refresh failed: %w", err)
	}
	return token, nil
}

// RefreshAPIToken は RefreshToken を使って BFF トークンをリフレッシュし、
// さらに Token Exchange で API AccessToken を取得して更新済みの SessionData を返す。
// ADR-0020 参照。
func RefreshAPIToken(ctx context.Context, sessionData redis.SessionData) (redis.SessionData, error) {
	newBFFToken, err := refreshBFFToken(ctx, sessionData.RefreshToken)
	if err != nil {
		return sessionData, fmt.Errorf("BFF token refresh failed: %w", err)
	}
	slog.Info("BFF token refreshed successfully")

	newAPIAccessToken, err := exchangeForAPIToken(ctx, newBFFToken.AccessToken)
	if err != nil {
		return sessionData, fmt.Errorf("token exchange after refresh failed: %w", err)
	}
	slog.Info("API access token obtained via token exchange after refresh")

	sessionData.AccessToken = newAPIAccessToken
	if newBFFToken.RefreshToken != "" {
		sessionData.RefreshToken = newBFFToken.RefreshToken
	}
	return sessionData, nil
}
