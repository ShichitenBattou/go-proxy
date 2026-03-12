package redis

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"

	"context"

	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func createClient() *redis.Client {
	return( redis.NewClient(&redis.Options{
		Addr: "redis:6379",
		Password: "",
		DB: 0,
	}))
}

func SetSession(sessionId string, userId string) error {
	rdb := createClient()
	defer rdb.Close()

	hashedSessionId := hashToken(sessionId)
	err := rdb.Set(ctx, "session:"+hashedSessionId, userId, 0).Err()
	if err != nil {
		return err
	}
	slog.Info("Session stored in Redis", "key", "session:"+hashedSessionId, "value", userId)
	return nil
}

func GetSessionValue(sessionId string) (string, error) {
	rdb := createClient()
	defer rdb.Close()

	hashedSessionId := hashToken(sessionId)
	sessionValue, err := rdb.Get(ctx, "session:"+hashedSessionId).Result()
	if errors.Is(err, redis.Nil) {
		return "", errors.New("Session not found in Redis")
	}
	slog.Info("Session retrieved from Redis", "key", "session:"+hashedSessionId, "value", sessionValue)
	return sessionValue, nil
}

func DeleteSession(sessionId string) error {
	rdb := createClient()
	defer rdb.Close()

	hashedSessionId := hashToken(sessionId)
	err := rdb.Del(ctx, "session:"+hashedSessionId).Err()
	if err != nil {
		return err
	}
	slog.Info("Session deleted from Redis", "key", "session:"+hashedSessionId)
	return nil
}