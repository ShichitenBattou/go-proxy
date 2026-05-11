# ADR-0020: リアクティブなトークンリフレッシュ（API 401 を契機とした自動更新）

## Status

提案中（Proposed） - 2026-05-11

## Context

### 問題

ADR-0015 で導入した Token Exchange フローにより、セッションには以下の 2 種類のトークンが保存されている（`redis.SessionData`）。

| フィールド | 内容 | 発行元 |
|---|---|---|
| `AccessToken` | API 向けアクセストークン（`aud=api`） | Keycloak Token Exchange |
| `RefreshToken` | BFF リフレッシュトークン（`aud=bff`） | Keycloak 認可コードフロー |

しかし現状、`RefreshToken` は一切使われていない。Keycloak のアクセストークンには有効期限（デフォルト: 5 分）があるため、有効期限切れ後のリクエストは次のフローを辿る：

```
ブラウザ → BFF → API
                 ↓
              401 Unauthorized（トークン失効）
                 ↓
BFF がそのまま透過 → ブラウザに 401 が届く
ユーザーは再ログインを強制される
```

`RefreshToken` を活用すれば、ユーザーへの影響なく透過的にトークンを更新できる。

### Token Exchange との組み合わせによる複雑性

ADR-0015 のフローにより、リフレッシュは以下の **2 ステップ** になる：

```
Step 1: RefreshToken → Keycloak → 新 BFF AccessToken + 新 RefreshToken
Step 2: 新 BFF AccessToken → Keycloak Token Exchange → 新 API AccessToken
```

単純な OAuth2 リフレッシュと異なり、Token Exchange の呼び出しが追加で必要になる。

### 実装戦略の選択肢

**案A: プロアクティブ（有効期限前に更新）**
- セッションにトークン有効期限を記録し、リクエスト前に期限切れ予測で更新する
- UX が最良だが、有効期限の追跡・クロックスキューへの対処が複雑

**案B: リアクティブ（API の 401 を契機に更新）**
- API から 401 が返ってきた時点でリフレッシュ → 同一リクエストをリトライ
- 実装がシンプルで障害モード（リフレッシュ失敗 → ログインリダイレクト）が明快

## Decision

**案B: リアクティブ方式** を採用する。

### 検出と処理の場所

`proxy/handler.go` の `ModifyResponse` で API からの `401 Unauthorized` を検出し、リフレッシュ + リトライを行う。

```
ModifyResponse で 401 を検出
    ↓
セッション Cookie からセッション ID を取得
    ↓
RefreshToken で新 BFF AccessToken を取得（Keycloak）
    ↓
新 BFF AccessToken で新 API AccessToken を取得（Token Exchange）
    ↓
Redis のセッションを新トークンで更新
    ↓
新 AccessToken を使って API へ直接リトライリクエスト
    ↓
成功: レスポンスをクライアントへ返す
失敗（RefreshToken 失効等）: セッション削除 → Keycloak ログインへリダイレクト
```

### 変更範囲

#### BFF: `auth/oauth2.go`

`RefreshToken` を使って新しい BFF アクセストークンを取得する関数を追加する。

```go
// refreshBFFToken は RefreshToken を使って新しい BFF アクセストークンと
// リフレッシュトークンを取得する。
func refreshBFFToken(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
    oauth2Config, err := getOAuth2Config()
    if err != nil {
        return nil, fmt.Errorf("failed to get oauth2 config: %w", err)
    }

    tokenSource := oauth2Config.TokenSource(ctx, &oauth2.Token{
        RefreshToken: refreshToken,
    })
    return tokenSource.Token()
}
```

#### BFF: `auth/oauth2.go`

リフレッシュから API アクセストークン取得までを一括で行うユーティリティ関数を追加する。

```go
// RefreshAPIToken は RefreshToken を使って API AccessToken を更新し、
// 新しい SessionData を返す。
func RefreshAPIToken(ctx context.Context, sessionData redis.SessionData) (redis.SessionData, error) {
    // Step 1: RefreshToken → 新 BFF AccessToken
    newToken, err := refreshBFFToken(ctx, sessionData.RefreshToken)
    if err != nil {
        return sessionData, fmt.Errorf("token refresh failed: %w", err)
    }

    // Step 2: 新 BFF AccessToken → 新 API AccessToken（Token Exchange）
    newAPIAccessToken, err := exchangeForAPIToken(ctx, newToken.AccessToken)
    if err != nil {
        return sessionData, fmt.Errorf("token exchange after refresh failed: %w", err)
    }

    // 更新された SessionData を返す（RefreshToken も更新）
    sessionData.AccessToken = newAPIAccessToken
    sessionData.RefreshToken = newToken.RefreshToken
    return sessionData, nil
}
```

#### BFF: `proxy/handler.go`

`ModifyResponse` に 401 検出・リフレッシュ・リトライロジックを追加する。

```go
modifyResponse := func(response *http.Response) error {
    // ... 既存の CORS ヘッダー処理 ...

    // API が 401 を返した場合、トークンリフレッシュを試みる
    if response.StatusCode == http.StatusUnauthorized {
        sessionCookie, err := response.Request.Cookie(cfg.SessionCookieName)
        if err != nil {
            // セッション Cookie がない場合はリフレッシュ不可
            return nil
        }

        sessionData, err := redis.GetSessionValue(sessionCookie.Value)
        if err != nil || sessionData.RefreshToken == "" {
            // セッションなし or RefreshToken なし → リフレッシュ不可
            return nil
        }

        // トークンリフレッシュ実行
        newSessionData, err := auth.RefreshAPIToken(response.Request.Context(), sessionData)
        if err != nil {
            slog.Warn("Token refresh failed, session will be invalidated", "error", err)
            redis.DeleteSession(sessionCookie.Value)
            // クライアントへ 401 を透過（ログインリダイレクトはクライアント側で処理）
            return nil
        }

        // Redis のセッションを新トークンで更新
        if err := redis.UpdateSession(sessionCookie.Value, newSessionData); err != nil {
            slog.Error("Failed to update session after token refresh", "error", err)
            return nil
        }

        // 新 AccessToken でリトライリクエストを直接発行
        retryResp, err := retryWithNewToken(response.Request, newSessionData.AccessToken, forwardHost)
        if err != nil {
            slog.Error("Retry request after token refresh failed", "error", err)
            return nil
        }

        // レスポンスを差し替える
        *response = *retryResp
        response.Header.Set("Access-Control-Allow-Origin", cfg.CORSAllowOrigin)
        response.Header.Set("Access-Control-Allow-Methods", cfg.CORSAllowMethods)
    }

    return nil
}
```

`retryWithNewToken` は新しいアクセストークンで API へ直接 HTTP リクエストを送り直すヘルパー関数。

### リフレッシュ失敗時の動作

| 失敗ケース | 動作 |
|---|---|
| `RefreshToken` 失効 | Redis からセッション削除、クライアントへ 401 を透過 |
| Token Exchange 失敗 | ログに記録、クライアントへ 401 を透過 |
| Redis 更新失敗 | ログに記録、クライアントへ 401 を透過（次回リクエストで再リフレッシュ） |
| リトライリクエスト失敗 | ログに記録、クライアントへ元の 401 を透過 |

### 並行リクエストへの対処

同一セッションで複数リクエストが同時に 401 を受けた場合、複数のリフレッシュが並行して走る可能性がある。初期実装では **ラストライターウィン**（後勝ち）を許容する。Keycloak のリフレッシュトークンローテーション設定が有効な場合、2 つ目以降のリフレッシュは失敗する可能性があるが、次回リクエスト時に更新済みセッションで成功するため、機能上の問題はない。

必要に応じて将来の ADR で Redis を使った分散ロックを検討する。

### 変更しないもの

- `redis.SessionData` — フィールド定義の変更不要
- `auth/callback_handler.go` — コールバック処理の変更不要
- `auth/login_handler.go` — ログインフローの変更不要
- セッションローテーション処理

## Consequences

**ポジティブ:**
- トークン失効によるセッション切断をユーザーが意識しなくて済む
- `RefreshToken` が初めて実際の用途で使われる
- 障害モードが明確（リフレッシュ失敗 → 401 透過 → クライアントがログインへ誘導）

**ネガティブ:**
- `ModifyResponse` 内でリフレッシュ + リトライの追加 HTTP リクエストが発生し、レイテンシが増加する
- Token Exchange フローのため、リフレッシュ 1 回につき Keycloak への HTTP リクエストが 2 回発生する
- 並行リクエストで複数リフレッシュが走る可能性がある（初期実装では許容）

## Implementation Notes

- `refreshBFFToken` は `golang.org/x/oauth2` の `TokenSource` を活用することで Keycloak への refresh_token グラントを簡潔に記述できる
- `retryWithNewToken` では `response.Request` を複製（`r.Clone(ctx)`）して `Authorization` ヘッダーを差し替え、`utils.GetInternalHTTPClient()` で API へ直接リクエストする
- リトライ時の Request-Id は新規 UUID を発行する
