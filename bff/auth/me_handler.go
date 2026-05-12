package auth

import (
	"bff/redis"
	"bff/setup"
	"encoding/json"
	"log/slog"
	"net/http"
)

// UserInfoResponse represents the response for /auth/me endpoint
type UserInfoResponse struct {
	Sub   string `json:"sub"`   // OIDC sub claim (user ID)
	Email string `json:"email"` // User email
	Name  string `json:"name"`  // User display name
}

// MeHandler returns the current user's information from the session
func MeHandler(w http.ResponseWriter, r *http.Request) {
	cfg := setup.GetConfig()
	slog.Info("Received /auth/me request", "url", r.URL.String(), "ip", r.RemoteAddr)

	// Get session cookie
	sessionCookie, err := r.Cookie(cfg.SessionCookieName)
	if err != nil {
		slog.Info("No session cookie found", "error", err)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	// Retrieve session data from Redis
	sessionData, err := redis.GetSessionValue(sessionCookie.Value)
	if err != nil {
		slog.Info("Session not found in Redis", "sessionId", sessionCookie.Value, "error", err)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	// Return user information (without tokens)
	response := UserInfoResponse{
		Sub:   sessionData.UserID,
		Email: sessionData.Email,
		Name:  sessionData.Name,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("Failed to encode response", "error", err)
	}
	slog.Info("Returned user info", "userId", sessionData.UserID, "email", sessionData.Email)
}
