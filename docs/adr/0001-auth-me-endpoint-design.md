# ADR-0001: GET /auth/me エンドポイントの設計

## Status
承認済み（Accepted） - 2026-04-15

## Context

現在、BFFには認証済みユーザーの情報を取得するエンドポイントが存在しません。フロントエンドが以下を実現するために `/auth/me` エンドポイントが必要です：

- ログイン状態の確認
- ユーザー名/メールアドレスの表示
- SPA初期化時の認証チェック

### 現在の実装状況

**Redisセッション構造:**
```go
// redis/redis.go:34-46
func SetSession(sessionId string, userId string) error
```
- キー: `session:<sha256(sessionId)>`
- 値: **文字列（userId のみ）**
- TTL: 30日（`SESSION_TTL`）

**問題点:**
現在のセッションには `userId` しか保存されていないため、`/auth/me` でユーザー情報を返すには以下のいずれかが必要：
1. セッションデータ構造を拡張してユーザー情報を保存
2. Keycloak UserInfo エンドポイントを都度呼び出し

## Decision

**選択: JSON構造体に拡張 + トークン保存あり + 最小限レスポンス**

### 1. セッションデータ構造

```go
type SessionData struct {
    UserID       string `json:"user_id"`    // OIDC sub claim
    Email        string `json:"email"`
    Name         string `json:"name"`

    // WARNING: Storing tokens in Redis without TLS/ACL has security risks.
    // - Risk: If Redis is compromised, attackers can impersonate users
    // - Mitigation: Ensure Redis is on isolated network, not publicly accessible
    // - Production TODO: Enable Redis TLS and ACL configuration
    AccessToken  string `json:"access_token,omitempty"`
    RefreshToken string `json:"refresh_token,omitempty"`
}
```

**理由:**
1. **BFFの本質**: 静的サイトでRefresh Token拡張を使えることがBFFの価値
2. **シームレスな認証**: Access Token期限切れ時、BFFがRefresh Tokenで自動更新
3. **セキュアなSPA**: フロントエンド（JavaScript）でトークンを扱わない

### 2. トークンの保存

**Access Token / Refresh Token を両方保存する**

**想定フロー:**
1. ユーザーがログイン
2. `/auth/callback` で Access Token + Refresh Token を取得
3. 両トークンをRedisのSessionDataに保存
4. セッションCookieのみをブラウザに返す（トークンは送らない）
5. `/api/*` リクエスト時、BFFがセッションからAccess Tokenを取得してバックエンドへ転送
6. Access Token期限切れ時、BFFがRefresh Tokenで自動更新

**セキュリティ考慮:**
- ⚠️ 現状: Redis ACL未使用、TLS未使用
- ✅ 緩和策: Docker内部ネットワークのみで通信（外部公開なし）
- 📝 コード内警告: トークン保存のリスクをコメントで明記
- 🔮 将来対応: 本番環境でRedis TLS/ACL有効化を推奨

### 3. レスポンス形式

**最小限:**
```json
{
  "sub": "keycloak-user-id",
  "email": "user@example.com",
  "name": "John Doe"
}
```

**実装方針:**
- `/auth/callback` でIDトークンからクレームを抽出し、SessionData（トークン含む）として保存
- `/auth/me` は Redis から SessionData を読み取り、ユーザー情報のみ（トークンなし）をレスポンス

## Consequences

**ポジティブ:**
- ✅ 静的サイトでもRefresh Token拡張が使える（BFFの本質）
- ✅ フロントエンドがトークンを扱わない（セキュア）
- ✅ Access Token期限切れ時の自動更新が可能
- ✅ 高速なレスポンス（Redis 1回読み取り）
- ✅ Keycloakダウンタイム時もセッション継続

**ネガティブ:**
- ⚠️ Redis侵害時のトークン漏洩リスク
  - **緩和策**: Docker内部ネットワークのみ、外部公開なし
  - **将来対応**: Redis TLS/ACL有効化
- ⚠️ ユーザー情報の更新が即座に反映されない
  - **対策**: 次回ログイン時に更新、セッションTTL調整
- ⚠️ セッションサイズ増加（トークン含む）
  - **影響**: 現状のTTL（30日）では許容範囲

**セキュリティTODO（別タスクで対応）:**
1. Cookie属性: `HttpOnly=true`, `Secure=true`, `SameSite=Strict`
2. Redis TLS有効化（本番環境推奨）
3. Redis ACL設定（本番環境推奨）

## Implementation Notes

- `/auth/me` エンドポイント実装
- `redis.SessionData` 構造体定義（セキュリティ警告コメント含む）
- `redis.SetSession` / `redis.GetSession` の型変更（string → SessionData）
- `/auth/callback` でトークンを SessionData に保存（別ADRで詳細設計）
