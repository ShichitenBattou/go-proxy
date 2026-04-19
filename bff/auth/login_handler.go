package auth

import (
	"bff/redis"
	"bff/setup"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	cfg := setup.GetConfig()
	slog.Info("Received login request", "url", r.URL.String(), "requestedHost", r.Host, "ip", r.RemoteAddr)

	// Get redirect_url from query parameters (validate early for better error messages)
	redirectURLStr := r.URL.Query().Get("redirect_url")
	if redirectURLStr == "" {
		redirectURLStr = "/" // Default to root page
	}

	// Security: Check for protocol-relative URLs (e.g., //evil.com)
	if strings.HasPrefix(redirectURLStr, "//") {
		slog.Error("Protocol-relative URL detected", "url", redirectURLStr)
		http.Error(w, "Invalid redirect URL", http.StatusBadRequest)
		return
	}

	// Security: Check for newline characters (HTTP header injection)
	if strings.ContainsAny(redirectURLStr, "\r\n") {
		slog.Error("Newline character detected in redirect URL", "url", redirectURLStr)
		http.Error(w, "Invalid redirect URL", http.StatusBadRequest)
		return
	}

	// Parse and validate redirect URL
	redirectURL, err := url.Parse(redirectURLStr)
	if err != nil {
		slog.Error("Invalid redirect_url parameter", "error", err, "url", redirectURLStr)
		http.Error(w, "Invalid redirect URL", http.StatusBadRequest)
		return
	}

	// Security: Prevent open redirect - if absolute URL, extract path only
	if redirectURL.IsAbs() {
		slog.Warn("Absolute URL in redirect_url, using path only", "url", redirectURLStr)
		redirectURL = &url.URL{Path: redirectURL.Path, RawQuery: redirectURL.RawQuery}
	}

	// Security: Validate redirect path against whitelist
	if !isAllowedRedirectPath(redirectURL.Path, cfg) {
		slog.Error("Redirect path not in whitelist", "path", redirectURL.Path)
		http.Error(w, "Redirect URL not allowed", http.StatusForbidden)
		return
	}

	oauth2Config, err := getOAuth2Config()
	if err != nil {
		slog.Error("Failed to get OAuth2 config", "error", err)
		http.Error(w, "Authentication service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Create a unique state value and store it in Redis with the redirect URL
	stateId := uuid.New()
	stateData := redis.StateData{
		RedirectURL: *redirectURL,
		CreatedAt:   time.Now().UTC(),
	}
	if err := redis.SetState(stateId, stateData); err != nil {
		slog.Error("Failed to set state in Redis", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	slog.Debug("Redirecting to Keycloak login page", "url", oauth2Config.AuthCodeURL(stateId.String()))

	http.Redirect(w, r, oauth2Config.AuthCodeURL(stateId.String()), http.StatusFound)
}
