package auth

import (
	"bff/redis"
	"bff/setup"
	"bff/utils"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type LoginRequest struct {
	RedirectURL string `json:"redirectUrl"`
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	cfg := setup.GetConfig()
	slog.Info("Received login request", "url", r.URL.String(), "requestedHost", r.Host, "ip", r.RemoteAddr)
	client := utils.GetInternalHTTPClient()
	res, err := client.Get(cfg.OIDCProviderURL)
	if err != nil {
		slog.Debug("Error connecting to Keycloak", "error", err)
	} else {
		slog.Debug("Successfully connected to Keycloak", "statusCode", res.StatusCode, "content", res.Body)
	}

	oauth2Config, err := getOAuth2Config()
	if err != nil {
		slog.Error("Failed to get OAuth2 config", "error", err)
		http.Error(w, "Authentication service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Create a unique state value and store it in Redis with the original redirect URL
	stateId := uuid.New()
	stateData := redis.StateData{
		RedirectURL: *r.URL,
		CreatedAt:   time.Now().UTC(),
	}
	if redis.SetState(stateId, stateData) != nil {
		slog.Error("Failed to set state in Redis", "error", err)
	}

	slog.Debug("Redirecting to Keycloak login page", "url", oauth2Config.AuthCodeURL(stateId.String()))

	http.Redirect(w, r, oauth2Config.AuthCodeURL(stateId.String()), http.StatusFound)
}
