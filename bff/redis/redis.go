package redis

import (
	"bff/setup"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func createClient() *redis.Client {
	cfg := setup.GetConfig()
	return redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
}

func SetSession(sessionId string, sessionData SessionData) error {
	cfg := setup.GetConfig()
	rdb := createClient()
	defer rdb.Close()

	// Marshal SessionData to JSON
	content, err := json.Marshal(sessionData)
	if err != nil {
		return err
	}

	hashedSessionId := hashToken(sessionId)
	key := cfg.SessionKeyPrefix + hashedSessionId
	err = rdb.Set(ctx, key, content, cfg.SessionTTL).Err()
	if err != nil {
		return err
	}
	slog.Info("Session stored in Redis", "key", key, "userId", sessionData.UserID, "ttl", cfg.SessionTTL)
	return nil
}

func GetSessionValue(sessionId string) (SessionData, error) {
	cfg := setup.GetConfig()
	rdb := createClient()
	defer rdb.Close()

	hashedSessionId := hashToken(sessionId)
	key := cfg.SessionKeyPrefix + hashedSessionId
	content, err := rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return SessionData{}, errors.New("Session not found in Redis")
	}
	if err != nil {
		return SessionData{}, err
	}

	var sessionData SessionData
	if err := json.Unmarshal([]byte(content), &sessionData); err != nil {
		return SessionData{}, err
	}
	slog.Info("Session retrieved from Redis", "key", key, "userId", sessionData.UserID)
	return sessionData, nil
}

func DeleteSession(sessionId string) error {
	cfg := setup.GetConfig()
	rdb := createClient()
	defer rdb.Close()

	hashedSessionId := hashToken(sessionId)
	key := cfg.SessionKeyPrefix + hashedSessionId
	err := rdb.Del(ctx, key).Err()
	if err != nil {
		return err
	}
	slog.Info("Session deleted from Redis", "key", key)
	return nil
}

// SessionData represents the session data stored in Redis.
// WARNING: Storing tokens in Redis without TLS/ACL has security risks.
// - Risk: If Redis is compromised, attackers can impersonate users
// - Mitigation: Ensure Redis is on isolated network, not publicly accessible
// - Production TODO: Enable Redis TLS and ACL configuration (see docs/adr/0001)
type SessionData struct {
	UserID       string `json:"user_id"`              // OIDC sub claim
	Email        string `json:"email"`                // User email
	Name         string `json:"name"`                 // User display name
	AccessToken  string `json:"access_token,omitempty"`  // OAuth2 access token
	RefreshToken string `json:"refresh_token,omitempty"` // OAuth2 refresh token
}

type StateData struct {
	RedirectURL url.URL   `json:"redirect_url"`
	CreatedAt   time.Time `json:"created_at"`
}

func SetState(state uuid.UUID, stateData StateData) error {
	cfg := setup.GetConfig()
	rdb := createClient()
	defer rdb.Close()

	content, err := json.Marshal(stateData)
	if err != nil {
		return err
	}

	key := cfg.StateKeyPrefix + state.String()
	err = rdb.Set(ctx, key, content, 0).Err()
	if err != nil {
		return err
	}
	slog.Info("State stored in Redis", "key", key, "value", stateData)
	return nil
}

func GetStateValue(state uuid.UUID) (StateData, error) {
	cfg := setup.GetConfig()
	rdb := createClient()
	defer rdb.Close()

	key := cfg.StateKeyPrefix + state.String()
	content, err := rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return StateData{}, errors.New("State not found in Redis")
	}
	var stateData StateData
	if err := json.Unmarshal([]byte(content), &stateData); err != nil {
		return StateData{}, err
	}
	slog.Info("State retrieved from Redis", "key", key, "value", stateData)
	return stateData, nil
}

func DeleteState(state uuid.UUID) error {
	cfg := setup.GetConfig()
	rdb := createClient()
	defer rdb.Close()

	key := cfg.StateKeyPrefix + state.String()
	err := rdb.Del(ctx, key).Err()
	if err != nil {
		return err
	}
	slog.Info("State deleted from Redis", "key", key)
	return nil
}