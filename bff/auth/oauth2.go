package auth

import (
	"bff/setup"
	"bff/utils"
	"context"
	"errors"
	"log/slog"

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
