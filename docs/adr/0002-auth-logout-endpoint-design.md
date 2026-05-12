# ADR-0002: POST /auth/logout エンドポイントの設計

## Status
承認済み（Accepted） - 2026-04-15

## Context

現在、ユーザーがログアウトする手段が存在しません。セキュリティ上、以下の理由でログアウト機能が必須です：

- ユーザーが明示的にセッションを終了できる
- 共有端末での使用後にセッションを削除
- セキュリティインシデント時の緊急対応

### 現在の実装状況

**セッション管理:**
- BFF: RedisにSessionData（ユーザー情報 + トークン）を保存
- Keycloak: OIDCセッションを管理（IdP側）

**問題点:**
ログアウト時に **BFFのセッション削除** と **KeycloakのOIDCセッション終了** の両方が必要

## Decision

**選択: グローバルログアウト（BFF + Keycloak）+ リダイレクト + ID Token保存**

### 1. ログアウトの範囲

**BFF + Keycloak両方（グローバルログアウト）**

OIDC [RP-Initiated Logout](https://openid.net/specs/openid-connect-rpinitiated-1_0.html) を使用:

```go
// 1. BFFセッション削除
redis.DeleteSession(sessionId)

// 2. Keycloak end_session_endpoint へリダイレクト
endSessionURL := fmt.Sprintf(
    "%s?id_token_hint=%s&post_logout_redirect_uri=%s&client_id=%s",
    provider.Endpoint().EndSessionEndpoint,
    sessionData.IDToken,
    url.QueryEscape("https://auth.local/"),
    cfg.OAuth2ClientID,
)
http.Redirect(w, r, endSessionURL, http.StatusFound)
```

**理由:**
- 完全なログアウト（IdPセッションも終了）
- セキュリティベストプラクティス
- SSO環境で他サービスもログアウト可能

### 2. ID Token の保存

**SessionData に `IDToken` フィールドを追加**

```go
type SessionData struct {
    UserID       string `json:"user_id"`
    Email        string `json:"email"`
    Name         string `json:"name"`
    IDToken      string `json:"id_token,omitempty"`       // OIDC ID Token
    AccessToken  string `json:"access_token,omitempty"`
    RefreshToken string `json:"refresh_token,omitempty"`
}
```

**理由:**
- OIDC RP-Initiated Logout で `id_token_hint` パラメータに使用
- 仕様準拠でセキュアなログアウト実現

### 3. レスポンス形式

**リダイレクト方式**

- Keycloak `end_session_endpoint` へリダイレクト
- Keycloakが `post_logout_redirect_uri` (`https://auth.local/`) へ戻す

**理由:**
- UXがシンプル（フロントエンドの追加実装不要）
- OIDCの標準フロー

### 4. エラーハンドリング

**BFFセッション削除を先に実行**

1. Redisからセッション削除（必須）
2. Cookie削除（必須）
3. Keycloakリダイレクト（ベストエフォート）

**Keycloakリダイレクト失敗時:**
- エラーログのみ記録
- ユーザーにはログアウト成功として扱う
- BFFセッションは削除済みなので最低限の安全は確保

### 5. post_logout_redirect_uri

**`https://auth.local/`**

- フロントエンドのトップページへリダイレクト
- ログアウト後、ユーザーは未認証状態でトップページを閲覧

## Implementation Notes

1. **SessionData拡張**: `IDToken` フィールド追加
2. **POST /auth/logout ハンドラー**:
   - セッションCookie取得
   - Redisからセッション削除
   - Cookie削除（MaxAge: -1）
   - OIDC Provider Discovery で `end_session_endpoint` 取得
   - Keycloakへリダイレクト（`id_token_hint`, `post_logout_redirect_uri` 含む）
3. **/auth/callback 修正**: ID Tokenを SessionData に保存（別ADRで詳細設計）

## Consequences

**ポジティブ:**
- ✅ 完全なログアウト（BFF + IdP）
- ✅ セキュリティベストプラクティス
- ✅ OIDC仕様に準拠（RP-Initiated Logout）
- ✅ SSO環境で他サービスもログアウト可能
- ✅ UXがシンプル（リダイレクトで自動遷移）

**ネガティブ:**
- ⚠️ Keycloak障害時にリダイレクト失敗の可能性
  - **緩和策**: BFFセッションは先に削除（最低限の安全は確保）
- ⚠️ ID Token の追加保存（セッションサイズ増加）
  - **影響**: 許容範囲（JWTは通常1-2KB程度）
- ⚠️ 実装の複雑性増加
  - **対策**: 既存の `getOAuth2Config()` を活用
