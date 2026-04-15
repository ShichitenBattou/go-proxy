package auth

import (
	"bff/redis"
	"bff/setup"
	"bff/utils"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/coreos/go-oidc/v3/oidc"
)

// LogoutHandler handles POST /auth/logout
// Implements OIDC RP-Initiated Logout (https://openid.net/specs/openid-connect-rpinitiated-1_0.html)
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	cfg := setup.GetConfig()
	slog.Info("Received /auth/logout request", "url", r.URL.String(), "ip", r.RemoteAddr)

	// Get session cookie
	sessionCookie, err := r.Cookie(cfg.SessionCookieName)
	if err != nil {
		slog.Info("No session cookie found during logout", "error", err)
		// Even if no session, redirect to post_logout_redirect_uri
		http.Redirect(w, r, "https://auth.local/", http.StatusFound)
		return
	}

	// Retrieve session data from Redis (to get ID token)
	sessionData, err := redis.GetSessionValue(sessionCookie.Value)
	var idToken string
	if err != nil {
		slog.Info("Session not found in Redis during logout", "sessionId", sessionCookie.Value, "error", err)
		// Continue with logout even if session not found
	} else {
		idToken = sessionData.IDToken
	}

	// Delete session from Redis (critical step)
	if err := redis.DeleteSession(sessionCookie.Value); err != nil {
		slog.Error("Failed to delete session from Redis", "sessionId", sessionCookie.Value, "error", err)
	} else {
		slog.Info("Session deleted from Redis", "sessionId", sessionCookie.Value)
	}

	// Delete session cookie
	http.SetCookie(w, &http.Cookie{
		Name:   cfg.SessionCookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1, // Delete cookie
	})
	slog.Info("Session cookie deleted")

	// Get OIDC provider's end_session_endpoint
	ctx := oidc.ClientContext(context.Background(), utils.GetInternalHTTPClient())
	provider, err := oidc.NewProvider(ctx, cfg.OIDCProviderURL)
	if err != nil {
		slog.Error("Failed to get OIDC provider for logout", "error", err)
		// BFF session is already deleted, redirect to frontend anyway
		http.Redirect(w, r, "https://auth.local/", http.StatusFound)
		return
	}

	// Build end_session_endpoint URL
	endSessionURL := provider.Endpoint().AuthURL // Keycloak uses same base
	// Note: go-oidc v3 doesn't expose EndSessionEndpoint directly, so we construct it
	// Keycloak's end_session_endpoint: {realm}/protocol/openid-connect/logout
	endSessionURL = cfg.OIDCProviderURL + "/protocol/openid-connect/logout"

	params := url.Values{}
	if idToken != "" {
		params.Set("id_token_hint", idToken)
	}
	params.Set("post_logout_redirect_uri", "https://auth.local/")
	params.Set("client_id", cfg.OAuth2ClientID)

	logoutURL := fmt.Sprintf("%s?%s", endSessionURL, params.Encode())
	slog.Info("Redirecting to Keycloak logout", "url", logoutURL)

	http.Redirect(w, r, logoutURL, http.StatusFound)
}
