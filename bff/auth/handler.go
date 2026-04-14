package auth

import (
	"bff/redis"
	"bff/setup"
	"bff/utils"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

func getOAuth2Config() *oauth2.Config {
	cfg := setup.GetConfig()
	ctx := oidc.ClientContext(context.Background(), utils.GetInternalHTTPClient())
	provider, err := oidc.NewProvider(ctx, cfg.OIDCProviderURL)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			slog.Error("OIDC provider initialization timed out. Ensure Keycloak is running and accessible", "providerURL", cfg.OIDCProviderURL, "error", err)
		} else {
			slog.Error("Failed to initialize OIDC provider", "error", err)
			// panic(err)
		}
	}

	oauth2Config := oauth2.Config{
		ClientID:     cfg.OAuth2ClientID,
		ClientSecret: cfg.OAuth2ClientSecret,
		RedirectURL:  cfg.OAuth2RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       cfg.OAuth2Scopes,
	}
	return &oauth2Config
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
	oauth2Config := getOAuth2Config()

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

func CallbackHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("Received callback request", "url", r.URL.String(), "requestedHost", r.Host, "ip", r.RemoteAddr)
	slog.Debug("Request parameters", "query", r.URL.Query().Encode())
	_, err := redis.GetStateValue(uuid.MustParse(r.URL.Query().Get("state")))
	if err != nil {
		slog.Error("Failed to get state from Redis", "error", err)
	}
	if redis.DeleteState(uuid.MustParse(r.URL.Query().Get("state"))) != nil {
		slog.Error("Failed to delete state from Redis", "error", err)
	}
	
	oauth2Config := getOAuth2Config()
	token, err := oauth2Config.Exchange(context.Background(), r.URL.Query().Get("code"))
	if err != nil {
		slog.Error("Failed to exchange code for token", "error", err)
		fmt.Fprint(w, "Authentication failed")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	slog.Info("Successfully exchanged code for token", "accessToken", token.AccessToken)

	

	fmt.Fprint(w, "Callback endpoint")
	w.WriteHeader(http.StatusNotImplemented)
}
