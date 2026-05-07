package auth

import (
	"bff/setup"
	"context"
	"fmt"
	"log/slog"
	"net/http"
)

// provisionUser は API の POST /users を呼び出し、ユーザーレコードを作成する。
// JWT の sub クレームは Bearer トークンから API 側が取得するため、リクエストボディは不要。
// API が一時的に停止している場合でも認証フローを失敗させないため、エラーはログのみに留める（ソフトフェイル）。
func provisionUser(ctx context.Context, apiAccessToken string) error {
	cfg := setup.GetConfig()
	url := fmt.Sprintf("http://%s/users", cfg.ProxyTarget)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create provision request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiAccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send provision request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("provision request returned unexpected status: %d", resp.StatusCode)
	}

	slog.Info("User provisioned successfully")
	return nil
}
