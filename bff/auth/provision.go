package auth

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// ProvisionFunc はユーザープロビジョニングを担う関数の型。
// テスト時にモックへ差し替えることで CallbackHandler の挙動を検証できる。
type ProvisionFunc func(ctx context.Context, apiAccessToken, targetHost string) error

// ProvisionUser は本番用のプロビジョニング関数。
// main.go から NewCallbackHandler へ渡して使用する。
func ProvisionUser(ctx context.Context, apiAccessToken, targetHost string) error {
	return provisionUser(ctx, apiAccessToken, targetHost)
}

// provisionHTTPClient は JIT プロビジョニング専用の HTTP クライアント。
// API は内部ネットワーク上の plain HTTP のため、カスタム TLS は不要。
// タイムアウトを設定し、API の無応答による goroutine ハングを防ぐ。
var provisionHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
}

// provisionUser は API の POST /users を呼び出し、ユーザーレコードを作成する。
// JWT の sub クレームは Bearer トークンから API 側が取得するため、リクエストボディは不要。
// targetHost には PROXY_TARGET 環境変数の値（例: "api:8081"）を渡す。
func provisionUser(ctx context.Context, apiAccessToken, targetHost string) error {
	url := fmt.Sprintf("http://%s/users", targetHost)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create provision request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiAccessToken)

	resp, err := provisionHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send provision request: %w", err)
	}
	defer resp.Body.Close()

	// レスポンスボディを必ず消費してコネクションを再利用可能にする
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("provision request returned unexpected status %d: %s", resp.StatusCode, string(body))
	}

	slog.Info("User provisioned successfully")
	return nil
}
