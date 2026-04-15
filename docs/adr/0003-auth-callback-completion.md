# ADR-0003: GET /auth/callback エンドポイントの完成

## Status
承認済み（Accepted） - 2026-04-15

## Context

現在、`/auth/callback` エンドポイントは認可コードをトークンに交換するところまで実装されていますが、セッション作成とユーザー情報の保存が未実装です（`bff/auth/handler.go:69-94`）。

### 現在の実装状況

**実装済み:**
```go
// 1. state トークンの検証・削除
stateData, err := redis.GetStateValue(uuid.MustParse(r.URL.Query().Get("state")))
redis.DeleteState(uuid.MustParse(r.URL.Query().Get("state")))

// 2. 認可コードをトークンに交換
token, err := oauth2Config.Exchange(context.Background(), r.URL.Query().Get("code"))
// token.AccessToken, token.RefreshToken, token.Extra("id_token")
```

**未実装:**
1. ID Token のパースと検証
2. ID Token からユーザー情報（クレーム）の抽出
3. SessionData の作成
4. Redis へセッション保存
5. セッションCookie の発行
6. 元のリダイレクト先（`StateData.RedirectURL`）へリダイレクト

### 依存関係

- SessionData構造体（ADR-0001, ADR-0002で定義）
- StateData構造体（既存）

```go
type SessionData struct {
    UserID       string `json:"user_id"`
    Email        string `json:"email"`
    Name         string `json:"name"`
    IDToken      string `json:"id_token,omitempty"`
    AccessToken  string `json:"access_token,omitempty"`
    RefreshToken string `json:"refresh_token,omitempty"`
}

type StateData struct {
    RedirectURL url.URL   `json:"redirect_url"`
    CreatedAt   time.Time `json:"created_at"`
}
```

## Decision

### 1. ID Token の取得と検証

**golang.org/x/oauth2 の `Token.Extra()` から取得**

```go
// Get raw ID Token from OAuth2 token response
rawIDToken, ok := token.Extra("id_token").(string)
if !ok {
    // Error: ID Token not found
}
```

**OIDC Verifier で検証**

```go
// Verify ID Token
verifier := provider.Verifier(&oidc.Config{ClientID: cfg.OAuth2ClientID})
idToken, err := verifier.Verify(ctx, rawIDToken)
if err != nil {
    // Error: ID Token verification failed
}
```

**理由:**
- ID Token の署名検証（セキュリティ）
- Issuer, Audience, Expiry のチェック
- OIDC仕様準拠

### 2. クレームの抽出

**ID Token からクレームを抽出**

```go
type IDTokenClaims struct {
    Sub           string `json:"sub"`
    Email         string `json:"email"`
    EmailVerified bool   `json:"email_verified"`
    Name          string `json:"name"`
}

var claims IDTokenClaims
if err := idToken.Claims(&claims); err != nil {
    // Error: Failed to parse claims
}
```

**必要なクレーム:**
- `sub`: ユーザーID（必須）
- `email`: メールアドレス
- `name`: 表示名

**オプション:**
- `email_verified`: メール確認済みフラグ（検証に使用可能）

### 3. SessionData の作成

```go
sessionData := redis.SessionData{
    UserID:       claims.Sub,
    Email:        claims.Email,
    Name:         claims.Name,
    IDToken:      rawIDToken,              // For logout
    AccessToken:  token.AccessToken,       // For API calls
    RefreshToken: token.RefreshToken,      // For token refresh
}
```

### 4. セッションの保存とCookie発行

```go
// Generate session ID
sessionID := uuid.New().String()

// Save to Redis
if err := redis.SetSession(sessionID, sessionData); err != nil {
    // Error handling
}

// Set session cookie
http.SetCookie(w, &http.Cookie{
    Name:     cfg.SessionCookieName,
    Value:    sessionID,
    Path:     "/",
    Secure:   cfg.SessionCookieSecure,
    HttpOnly: true,  // TODO: Add to config
    SameSite: http.SameSiteStrictMode,  // TODO: Add to config
})
```

**Cookie属性:**
- `HttpOnly`: JavaScript からアクセス不可（XSS対策）
- `Secure`: HTTPS通信のみ
- `SameSite`: CSRF対策

### 5. リダイレクト先の決定

**StateData.RedirectURL を使用**

```go
// Redirect to original URL
http.Redirect(w, r, stateData.RedirectURL.String(), http.StatusFound)
```

**フォールバック:**
- StateDataが取得できない場合は `/` へリダイレクト

### 6. エラーハンドリング

**エラーシナリオ:**

| エラー | ステータス | レスポンス |
|--------|----------|-----------|
| state不正（CSRF） | 403 Forbidden | `"Invalid state parameter"` |
| トークン交換失敗 | 401 Unauthorized | `"Authentication failed"` |
| ID Token検証失敗 | 401 Unauthorized | `"Invalid ID token"` |
| Redis保存失敗 | 500 Internal Server Error | `"Failed to create session"` |

**ログ出力:**
- 全エラーをログに記録（デバッグ用）
- Access Token / Refresh Token の値はログに**出力しない**（セキュリティ）

## Implementation Notes

### 実装ステップ

1. `CallbackHandler` の完成:
   - ID Token 検証・パース
   - SessionData 作成
   - Redis保存
   - Cookie発行
   - リダイレクト

2. Cookie設定の追加（`setup/config.go`）:
   - `SessionCookieHttpOnly` (デフォルト: `true`)
   - `SessionCookieSameSite` (デフォルト: `Strict`)

3. テスト作成:
   - 正常系（Keycloakモック必要 → 統合テスト）
   - エラー系（state不正、トークン交換失敗）

### セキュリティ考慮事項

1. **ID Token検証**: 必ず実施（署名、issuer, audience, expiry）
2. **state検証**: CSRF対策として必須
3. **Cookie属性**: HttpOnly, Secure, SameSite を適切に設定
4. **トークンのログ出力禁止**: Access Token, Refresh Token は機密情報

## Consequences

**ポジティブ:**
- ✅ 完全なOIDC認証フロー実装
- ✅ セキュアなセッション管理（ID Token検証）
- ✅ Cookie属性でXSS/CSRF対策（HttpOnly, Secure, SameSite=Strict）
- ✅ Refresh Token でシームレスな認証継続
- ✅ 元のリダイレクト先へ自動復帰（UX向上）

**ネガティブ:**
- ⚠️ テストの複雑性（OIDC Providerモックが必要）
  - **対策**: 統合テストで実施（Keycloak起動済み環境）、ユニットテストはエラー系のみ
- ⚠️ Cookie設定の追加（後方互換性なし）
  - **影響**: 新規実装なので問題なし

**決定事項（承認済み）:**
- クレームは `sub`, `email`, `name` のみ（roles/groups不要）
- Cookie属性: `HttpOnly=true`, `Secure`, `SameSite=Strict`
- リダイレクト先: `StateData.RedirectURL`
