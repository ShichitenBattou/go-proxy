# ADR-0009: セッションIDローテーション戦略の見直し

## Status
承認済み（Accepted） - 2026-04-19

## Context

### 現在の実装

`bff/proxy/handler.go` において、すべての API レスポンスごとにセッションIDをローテーション（削除 + 再作成）している。

**proxy/handler.go:76-106:**
```go
modifyResponse := func(response *http.Response) error {
    // ...

    // Session rotation: delete old session and create new one
    var oldSessionData redis.SessionData
    if existedSessionId == nil {
        slog.Info("No existing session found, skipping session rotation")
        return nil
    } else {
        // Retrieve old session data before deletion
        sessionCookie, _ := response.Request.Cookie(cfg.SessionCookieName)
        if sessionCookie != nil {
            oldSessionData, _ = redis.GetSessionValue(sessionCookie.Value)
        }
        redis.DeleteSession(*existedSessionId)  // 👈 毎レスポンスで削除
    }

    // Create new session with rotated ID
    newSessionID := uuid.New()
    // ...
    err := redis.SetSession(newSessionID.String(), oldSessionData)  // 👈 毎レスポンスで作成
    // ...
}
```

### 問題点

#### 1. **過剰なセキュリティ対策（Over-engineering）**

**セッション固定攻撃（Session Fixation Attack）の防止は、ログイン時のローテーションで十分です。**

**セッションIDローテーションが必要なタイミング:**
- ✅ **認証時（ログイン成功時）** — 攻撃者が事前に設定したセッションIDを無効化
- ⚠️ 権限昇格時（必要に応じて）— ユーザーが管理者権限を取得した場合など
- ❌ **通常のAPIリクエストごと** — セキュリティ効果はほぼゼロ、オーバーヘッドのみ

**理由:**
- セッション固定攻撃は、攻撃者が**ログイン前**に被害者のセッションIDを設定し、被害者がログインした後にそのIDを使って認証済みセッションを乗っ取る攻撃
- **ログイン時にセッションIDをローテーション**すれば、攻撃者が設定したIDは無効化される
- 通常のAPIリクエストごとにローテーションしても、追加のセキュリティ効果はない

**OWASP の推奨:**
- [OWASP Session Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html#renew-the-session-id-after-any-privilege-level-change)
  > "Renew the session ID after any privilege level change within the application"
  > （権限レベルの変更時にセッションIDを更新する）

#### 2. **パフォーマンスへの悪影響（Critical）**

**毎リクエストで Redis への削除 + 書き込みが発生:**
- `redis.DeleteSession(*existedSessionId)` — DELETE コマンド
- `redis.SetSession(newSessionID.String(), oldSessionData)` — SET コマンド

**影響:**
- SPA では 1ページ表示に複数の API リクエストが発生（例: `/api/users`, `/api/posts`, `/api/notifications`）
- 各リクエストで 2回の Redis 操作 → 高負荷時に Redis がボトルネックに
- Redis のコネクション作成/クローズも毎回発生（`redis/redis.go:25` — コネクションプールなし）

**測定例（仮定）:**
- 100 req/sec のトラフィック → 200 Redis ops/sec（削除 + 作成）
- 1000 req/sec → 2000 Redis ops/sec

#### 3. **レースコンディションのリスク（High）**

**CLAUDE.md でも指摘されている:**
> "Session rotation: Every API response triggers session deletion + creation, which may cause race conditions under high concurrency"

**シナリオ:**
1. ブラウザが `/api/users` と `/api/posts` を並行リクエスト（同じセッションID: `session-A`）
2. `/api/users` のレスポンスが先に返る → `session-A` を削除 + `session-B` を作成 + `Set-Cookie: session-B`
3. `/api/posts` のリクエストが BFF に到着（まだ Cookie は `session-A`）
4. BFF が Redis で `session-A` を検証 → **すでに削除済み** → 401 Unauthorized

**結果:**
- ユーザーがランダムに認証エラーを経験
- フロントエンドが `/auth/login` にリダイレクト
- UX の悪化、ユーザークレーム

#### 4. **クライアント側の Cookie 同期問題（High）**

**ブラウザの Cookie 更新タイミング:**
- `Set-Cookie` ヘッダーはレスポンス受信後に反映される
- 並行リクエスト時、最初のレスポンスの `Set-Cookie` が反映される前に、次のリクエストが送信される

**シナリオ（タイムライン）:**
```
T0: Browser → BFF: GET /api/users (Cookie: session-A)
T1: Browser → BFF: GET /api/posts (Cookie: session-A) ← 並行リクエスト
T2: BFF → Browser: Response /api/users (Set-Cookie: session-B)
T3: BFF processes GET /api/posts (Cookie: session-A) ← すでに session-A は削除済み
T4: BFF → Browser: 401 Unauthorized
```

**結果:**
- 並行リクエストの2番目以降が失敗する
- フロントエンドでリトライロジックが必要になる（複雑化）

#### 5. **デバッグの困難さ（Medium）**

**ログ追跡が困難:**
- 各リクエストでセッションIDが変わるため、ユーザーの行動を追跡しにくい
- 例: ユーザー A の `/api/users` → セッションID: `abc123`、次の `/api/posts` → セッションID: `def456`
- ログから「同一ユーザーの一連の操作」を特定するには `UserID` を使う必要がある

#### 6. **Redis メモリ使用量の増加（Low）**

**セッションデータの一時的な重複:**
- 古いセッションを削除する前に、新しいセッションを作成すると、一瞬だけ2つのセッションが存在
- 実装上は削除→作成の順なので問題ないが、エラー時には古いセッションが残る可能性

## Decision

### 提案: ログイン時のみセッションIDをローテーション

**変更内容:**
1. **`proxy/handler.go` からセッションローテーションロジックを削除**
   - `modifyResponse` 関数から `DeleteSession` + `SetSession` を削除
   - セッション検証（`GetSessionValue`）のみ残す

2. **`auth/callback_handler.go` でセッションIDをローテーション（新規実装）**
   - OAuth2 callback でトークン取得後、セッション作成
   - **ログイン時のみ**セッションIDが変更される

3. **セッション有効期限の更新（オプション）**
   - 毎リクエストで TTL を更新し、アクティブなセッションを延長
   - または、最終アクセス時刻を記録し、一定期間経過後に期限切れ

### 実装詳細

#### 変更1: `proxy/handler.go` — セッションローテーションを削除

**変更前（76-106行目）:**
```go
modifyResponse := func(response *http.Response) error {
    slog.Info("Received response", "statusCode", response.StatusCode, "url", response.Request.URL.String())
    response.Header.Set("Access-Control-Allow-Origin", cfg.CORSAllowOrigin)
    response.Header.Set("Access-Control-Allow-Methods", cfg.CORSAllowMethods)

    // Session rotation: delete old session and create new one
    var oldSessionData redis.SessionData
    if existedSessionId == nil {
        slog.Info("No existing session found, skipping session rotation")
        return nil
    } else {
        // Retrieve old session data before deletion
        sessionCookie, _ := response.Request.Cookie(cfg.SessionCookieName)
        if sessionCookie != nil {
            oldSessionData, _ = redis.GetSessionValue(sessionCookie.Value)
        }
        redis.DeleteSession(*existedSessionId)
    }

    // Create new session with rotated ID
    newSessionID := uuid.New()
    cookieValue := fmt.Sprintf("%s=%s", cfg.SessionCookieName, newSessionID.String())
    if cfg.SessionCookieSecure {
        cookieValue += "; Secure"
    }
    response.Header.Set("Set-Cookie", cookieValue)

    // Store the session with existing SessionData (rotation)
    err := redis.SetSession(newSessionID.String(), oldSessionData)
    if err != nil {
        slog.Error("Error setting session in Redis", "error", err)
    } else {
        slog.Info("Session rotated in Redis", "key", "session:"+newSessionID.String(), "userId", oldSessionData.UserID)
    }

    return nil
}
```

**変更後（シンプル化）:**
```go
modifyResponse := func(response *http.Response) error {
    slog.Info("Received response", "statusCode", response.StatusCode, "url", response.Request.URL.String())
    response.Header.Set("Access-Control-Allow-Origin", cfg.CORSAllowOrigin)
    response.Header.Set("Access-Control-Allow-Methods", cfg.CORSAllowMethods)

    // セッションローテーションは削除（ログイン時のみ実施）
    // セッション有効期限の更新はオプション（後述）

    return nil
}
```

**または、セッションTTL更新を追加（オプション）:**
```go
modifyResponse := func(response *http.Response) error {
    slog.Info("Received response", "statusCode", response.StatusCode, "url", response.Request.URL.String())
    response.Header.Set("Access-Control-Allow-Origin", cfg.CORSAllowOrigin)
    response.Header.Set("Access-Control-Allow-Methods", cfg.CORSAllowMethods)

    // セッションTTLを更新（アクティブなセッションを延長）
    if existedSessionId != nil {
        sessionCookie, _ := response.Request.Cookie(cfg.SessionCookieName)
        if sessionCookie != nil {
            sessionData, err := redis.GetSessionValue(sessionCookie.Value)
            if err == nil {
                // TTLのみ更新（セッションIDは変更しない）
                redis.SetSession(sessionCookie.Value, sessionData)
                slog.Debug("Session TTL refreshed", "sessionId", sessionCookie.Value)
            }
        }
    }

    return nil
}
```

#### 変更2: `auth/callback_handler.go` — ログイン時のセッションローテーション

**変更前（auth/callback_handler.go:69 — 未実装部分）:**
```go
// TODO: Create session here
w.WriteHeader(http.StatusNotImplemented)
w.Write([]byte("Callback handler: token exchange successful, but session creation not implemented"))
```

**変更後（セッション作成 + Cookie設定）:**
```go
// Create new session with user data
sessionID := uuid.New()
sessionData := redis.SessionData{
    UserID:       userInfo.Sub,  // User ID from Keycloak
    AccessToken:  token.AccessToken,
    RefreshToken: token.RefreshToken,
    ExpiresAt:    time.Now().Add(time.Duration(token.ExpiresIn) * time.Second),
}

err = redis.SetSession(sessionID.String(), sessionData)
if err != nil {
    slog.Error("Failed to create session in Redis", "error", err)
    http.Error(w, "Failed to create session", http.StatusInternalServerError)
    return
}

// Set secure session cookie
cookieValue := fmt.Sprintf("%s=%s; Path=/; HttpOnly; SameSite=Lax", cfg.SessionCookieName, sessionID.String())
if cfg.SessionCookieSecure {
    cookieValue += "; Secure"
}
http.SetCookie(w, &http.Cookie{
    Name:     cfg.SessionCookieName,
    Value:    sessionID.String(),
    Path:     "/",
    HttpOnly: true,
    Secure:   cfg.SessionCookieSecure,
    SameSite: http.SameSiteLaxMode,
})

slog.Info("Session created after login", "sessionId", sessionID.String(), "userId", userInfo.Sub)

// Redirect to original URL
http.Redirect(w, r, stateData.RedirectURL, http.StatusFound)
```

**重要な変更点:**
- セッション作成は**ログイン時（認証成功時）のみ**
- Cookie に `HttpOnly`, `SameSite=Lax` を設定（XSS, CSRF 対策）
- `cfg.SessionCookieSecure` で本番環境は `Secure` 属性を強制

#### 変更3: `rewrite` 関数 — `existedSessionId` 変数の削除

**変更前（proxy/handler.go:29, 44, 47）:**
```go
var existedSessionId *string  // 👈 modifyResponse で使うためのグローバル変数

rewrite := func(request *httputil.ProxyRequest) {
    // ...
    if err != nil {
        // ...
        existedSessionId = nil
    } else {
        // ...
        existedSessionId = &hashedSessionId
        // ...
    }
}
```

**変更後:**
```go
// existedSessionId 変数は不要（削除）

rewrite := func(request *httputil.ProxyRequest) {
    sessionID, err := request.In.Cookie(cfg.SessionCookieName)
    if err != nil {
        slog.Error("Error getting cookie", "error", err)
        // セッションがない場合は何もしない（/auth/login へリダイレクトはミドルウェアで処理）
    } else {
        slog.Info("Received request with cookie", "cookie", sessionID)
    }

    // Check if the session ID exists in Redis
    hashedSessionId := hashToken(sessionID.Value)
    sessionData, err := redis.GetSessionValue(sessionID.Value)
    if err != nil {
        slog.Info("Session not found in Redis", "sessionId", sessionID.Value)
    } else {
        slog.Info("Session found in Redis", "sessionId", sessionID.Value)

        // Add Authorization header with AccessToken
        if sessionData.AccessToken != "" {
            request.Out.Header.Set("Authorization", "Bearer "+sessionData.AccessToken)
        } else {
            slog.Warn("AccessToken is empty in session", "sessionId", sessionID.Value)
        }
    }

    // Proxy設定（既存のまま）
    request.Out.Header["X-Forwarded-For"] = request.In.Header["X-Forwarded-For"]
    request.Out.URL.Scheme = "http"
    request.Out.URL.Host = forwardHost
    request.Out.Header.Set("Request-Id", uuid.New().String())
    urlPath := strings.TrimPrefix(request.In.URL.Path, "/api")
    if urlPath == "" || urlPath[0] != '/' {
        urlPath = "/" + urlPath
    }
    request.Out.URL.Path = path.Clean(urlPath)
    request.SetXForwarded()
    slog.Info("Proxying request", "url", request.Out.URL.String(), "requestedHost", request.In.Host, "ip", request.In.RemoteAddr)
}
```

### セッションTTL更新戦略（オプション）

**3つの選択肢:**

| 戦略 | メリット | デメリット | 推奨度 |
|------|---------|----------|--------|
| **1. TTL更新なし** | シンプル、Redis負荷最小 | セッション期限が固定（例: 30日後に強制ログアウト） | 🟢 推奨（初期実装） |
| **2. 毎リクエストでTTL更新** | アクティブユーザーはログアウトしない | Redis書き込みが発生（現状と同じ負荷） | 🟡 要検討 |
| **3. 一定期間ごとにTTL更新** | バランス型（例: 1時間に1回） | 実装が複雑（最終更新時刻を記録） | 🟡 要検討 |

**推奨: 戦略1（TTL更新なし）**
- シンプルで理解しやすい
- Redis 負荷を最小化
- セッション期限（例: 30日）は十分に長い
- 必要に応じて、後から戦略2/3に変更可能

## Implementation Notes

### 変更箇所

**1. proxy/handler.go:**
- `existedSessionId` 変数を削除（行29）
- `rewrite` 関数から `existedSessionId` への代入を削除（行44, 47）
- `modifyResponse` 関数からセッションローテーションロジックを削除（行76-106）
- CORS ヘッダー設定のみ残す

**2. auth/callback_handler.go:**
- セッション作成ロジックを追加（行69 — 現在は未実装）
- セキュアな Cookie 設定（`HttpOnly`, `SameSite=Lax`, `Secure`）

**3. redis/redis.go:**
- 変更不要（既存の `SetSession`, `GetSessionValue` を使用）

**4. setup/config.go:**
- Cookie 設定を追加（必要に応じて）:
  - `SessionCookieHttpOnly bool` （デフォルト: `true`）
  - `SessionCookieSameSite string` （デフォルト: `Lax`）

### テストケース

**proxy/handler_test.go:**
```go
// セッションローテーションが行われないことを確認
func TestProxyHandler_NoSessionRotation(t *testing.T) {
    // Setup: セッションを作成
    sessionID := "test-session-123"
    redis.SetSession(sessionID, redis.SessionData{UserID: "user@example.com"})

    // Request: Cookie付きでAPIリクエスト
    req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
    req.AddCookie(&http.Cookie{Name: "Session-Id", Value: sessionID})

    rr := httptest.NewRecorder()
    handler.ServeHTTP(rr, req)

    // Assert: Set-Cookieヘッダーが返されない
    if rr.Header().Get("Set-Cookie") != "" {
        t.Errorf("Expected no Set-Cookie header, got: %s", rr.Header().Get("Set-Cookie"))
    }

    // Assert: 元のセッションIDがRedisに残っている
    _, err := redis.GetSessionValue(sessionID)
    if err != nil {
        t.Errorf("Expected session to remain in Redis, got error: %v", err)
    }
}

// 並行リクエストでレースコンディションが発生しないことを確認
func TestProxyHandler_ConcurrentRequests(t *testing.T) {
    sessionID := "test-session-concurrent"
    redis.SetSession(sessionID, redis.SessionData{UserID: "user@example.com"})

    var wg sync.WaitGroup
    errors := make(chan error, 10)

    // 10並行リクエスト
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
            req.AddCookie(&http.Cookie{Name: "Session-Id", Value: sessionID})
            rr := httptest.NewRecorder()
            handler.ServeHTTP(rr, req)

            if rr.Code != http.StatusOK {
                errors <- fmt.Errorf("Expected 200, got %d", rr.Code)
            }
        }()
    }

    wg.Wait()
    close(errors)

    // Assert: すべてのリクエストが成功
    for err := range errors {
        t.Error(err)
    }
}
```

**auth/callback_handler_test.go:**
```go
// ログイン時にセッションが作成されることを確認
func TestCallbackHandler_SessionCreation(t *testing.T) {
    // Setup: モックKeycloak
    // ...

    // Request: OAuth2 callback
    req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=test-code&state=test-state", nil)
    rr := httptest.NewRecorder()
    CallbackHandler(rr, req)

    // Assert: Set-Cookieヘッダーが返される
    cookies := rr.Result().Cookies()
    if len(cookies) == 0 {
        t.Fatal("Expected Set-Cookie header")
    }

    sessionCookie := cookies[0]
    if sessionCookie.Name != "Session-Id" {
        t.Errorf("Expected cookie name 'Session-Id', got: %s", sessionCookie.Name)
    }

    // Assert: セッションがRedisに保存されている
    sessionData, err := redis.GetSessionValue(sessionCookie.Value)
    if err != nil {
        t.Fatalf("Expected session in Redis, got error: %v", err)
    }

    if sessionData.UserID == "" {
        t.Error("Expected UserID in session")
    }
}

// セッションCookieがセキュアな設定になっていることを確認
func TestCallbackHandler_SecureCookie(t *testing.T) {
    // ...
    cookies := rr.Result().Cookies()
    sessionCookie := cookies[0]

    // Assert: HttpOnly
    if !sessionCookie.HttpOnly {
        t.Error("Expected HttpOnly=true")
    }

    // Assert: SameSite=Lax
    if sessionCookie.SameSite != http.SameSiteLaxMode {
        t.Errorf("Expected SameSite=Lax, got: %v", sessionCookie.SameSite)
    }

    // Assert: Secure (in production)
    if cfg.SessionCookieSecure && !sessionCookie.Secure {
        t.Error("Expected Secure=true in production")
    }
}
```

### 実装ステップ

1. **proxy/handler.go の修正**
   - `existedSessionId` 変数を削除
   - `modifyResponse` 関数をシンプル化（CORSヘッダー設定のみ）
   - `rewrite` 関数から `existedSessionId` への参照を削除

2. **auth/callback_handler.go の修正**
   - セッション作成ロジックを追加（TODO 部分を実装）
   - セキュアな Cookie 設定

3. **setup/config.go の修正（オプション）**
   - Cookie 属性の設定項目を追加

4. **テストの追加**
   - `proxy/handler_test.go` — セッションローテーションが発生しないことを確認
   - `auth/callback_handler_test.go` — ログイン時のセッション作成を確認

5. **手動テスト**
   - ログインフロー → セッション作成確認
   - 複数のAPIリクエスト → セッションIDが変わらないことを確認
   - 並行リクエスト → レースコンディションが発生しないことを確認

## Consequences

### ポジティブ

- ✅ **パフォーマンス大幅改善**: Redis 書き込みが劇的に減少（毎リクエスト → ログイン時のみ）
- ✅ **レースコンディション解消**: 並行リクエストでの認証エラーがなくなる
- ✅ **UX改善**: ランダムなログアウトが発生しない
- ✅ **デバッグ容易**: セッションIDが変わらないため、ログ追跡が簡単
- ✅ **コード簡素化**: `proxy/handler.go` の複雑なローテーションロジックを削除
- ✅ **セキュリティ維持**: ログイン時のローテーションでセッション固定攻撃を防止
- ✅ **業界標準に準拠**: OWASP の推奨事項に従う

### ネガティブ

- ⚠️ **セキュリティの誤解**: 「毎回ローテーション = より安全」という誤解を解く必要がある
- ⚠️ **既存の認識との相違**: 現在の実装を「セキュリティ機能」として認識している場合、削除に抵抗感がある可能性
- ⚠️ **テストの追加**: 新しいテストケースの作成が必要

### リスク評価

- 🟢 **セキュリティリスク: なし**
  - セッション固定攻撃対策はログイン時のローテーションで十分
  - OWASP の推奨事項に準拠
  - 業界標準のセッション管理手法
- 🟢 **実装リスク: 低**
  - コードの削減が主体（追加機能ではない）
  - テストカバレッジで品質担保
- 🟢 **運用リスク: なし**
  - パフォーマンス改善のみ、機能的な変更なし

### 代替案との比較

| 案 | メリット | デメリット |
|---------|---------|----------|
| **現状維持（毎レスポンスローテーション）** | なし | 🔴 パフォーマンス低下、レースコンディション、UX悪化 |
| **毎リクエストでTTL更新のみ** | アクティブセッション延長 | Redis書き込みは発生（現状と同じ負荷） |
| **提案案（ログイン時のみローテーション）** | パフォーマンス最大化、レースなし | セッション期限が固定 |
| **一定期間ごとにローテーション** | セキュリティと性能のバランス | 実装が複雑、効果は限定的 |

### 決定事項

1. **proxy/handler.go からセッションローテーションを削除** — 毎レスポンスでの削除+作成を撤廃
2. **auth/callback_handler.go でログイン時にセッション作成** — 認証成功時にセッションIDを発行
3. **セキュアな Cookie 設定** — `HttpOnly`, `SameSite=Lax`, `Secure`（本番環境）
4. **セッションTTL更新なし（初期実装）** — シンプルさを優先、必要に応じて後で追加

### セキュリティ基準

**OWASP Session Management Cheat Sheet 準拠:**
- ✅ ログイン時のセッションIDローテーション（Session Fixation 対策）
- ✅ HttpOnly 属性（XSS 対策）
- ✅ SameSite 属性（CSRF 対策）
- ✅ Secure 属性（HTTPS 強制）
- ✅ 十分なセッション有効期限（例: 30日）

**推奨される本番環境設定:**
```bash
# .env (本番環境)
SESSION_TTL=720h                      # 30日
SESSION_COOKIE_NAME=Session-Id
SESSION_COOKIE_SECURE=true            # HTTPS必須
SESSION_COOKIE_HTTP_ONLY=true         # XSS対策
SESSION_COOKIE_SAME_SITE=Lax          # CSRF対策
```

## References

- [OWASP Session Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)
- [RFC 6749 Section 10.12: Session Fixation](https://datatracker.ietf.org/doc/html/rfc6749#section-10.12)
- [MDN: Set-Cookie](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Set-Cookie)

## Notes

- `bff/CLAUDE.md` に記載されている "Session rotation: may cause race conditions" の問題を解決
- `auth/callback_handler.go:69` の TODO（セッション作成未実装）を同時に解決
- この変更により、BFF の実装が業界標準のセッション管理パターンに準拠
