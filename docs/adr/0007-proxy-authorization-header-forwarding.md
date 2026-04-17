# ADR-0007: Proxy Handler での Authorization ヘッダー付与

## Status
提案中（Proposed） - 2026-04-17

## Context

### 現在の実装

`bff/proxy/handler.go` において：

**問題点:**
1. Redis からセッションデータ（`redis.SessionData`）を取得している（handler.go:40）
2. セッションデータには `AccessToken` が含まれている（redis/redis.go:103）
3. しかし、API側へのリクエストに `Authorization` ヘッダーを付与していない
4. そのため、API側でユーザー認証情報を利用できない

**現在のコード:**
```go
// proxy/handler.go:31-61
rewrite := func(request *httputil.ProxyRequest) {
    sessionID, err := request.In.Cookie(cfg.SessionCookieName)
    // ...
    _, err = redis.GetSessionValue(sessionID.Value)  // SessionData を取得するが使用していない
    // ...
    request.Out.URL.Scheme = "http"
    request.Out.URL.Host = forwardHost
    request.Out.Header.Set("Request-Id", uuid.New().String())
    // Authorization ヘッダーの設定がない
}
```

**SessionData の構造:**
```go
// redis/redis.go:98-105
type SessionData struct {
    UserID       string `json:"user_id"`
    Email        string `json:"email"`
    Name         string `json:"name"`
    IDToken      string `json:"id_token,omitempty"`
    AccessToken  string `json:"access_token,omitempty"`  // 👈 これをAPI側に渡す必要がある
    RefreshToken string `json:"refresh_token,omitempty"`
}
```

### 要件

API側は `Authorization: Bearer <AccessToken>` ヘッダーを期待している。

## Decision

### Redis から取得した AccessToken を API に転送する

**変更内容:**
1. `rewrite` 関数内で `redis.GetSessionValue()` の戻り値（`SessionData`）を変数に格納
2. `SessionData.AccessToken` を取得
3. `Authorization: Bearer <token>` ヘッダーとしてAPIリクエストに付与

**実装:**
```go
rewrite := func(request *httputil.ProxyRequest) {
    sessionID, err := request.In.Cookie(cfg.SessionCookieName)
    if err != nil {
        slog.Error("Error getting cookie", "error", err)
    } else {
        slog.Info("Received request with cookie", "cookie", sessionID)
    }

    // Check if the session ID exists in Redis
    hashedSessionId := hashToken(sessionID.Value)
    sessionData, err := redis.GetSessionValue(sessionID.Value)  // 👈 変更: 戻り値を受け取る
    if err != nil {
        slog.Info("Session not found in Redis", "sessionId", sessionID.Value)
        existedSessionId = nil
    } else {
        slog.Info("Session found in Redis", "sessionId", sessionID.Value)
        existedSessionId = &hashedSessionId

        // 👈 追加: AccessToken を Authorization ヘッダーに設定
        if sessionData.AccessToken != "" {
            request.Out.Header.Set("Authorization", "Bearer "+sessionData.AccessToken)
        }
    }

    // 既存のヘッダー設定
    request.Out.Header["X-Forwarded-For"] = request.In.Header["X-Forwarded-For"]
    request.Out.URL.Scheme = "http"
    request.Out.URL.Host = forwardHost
    request.Out.Header.Set("Request-Id", uuid.New().String())
    // ...
}
```

### AccessToken が空の場合の処理

**選択肢A: ヘッダーを設定しない（スキップ）**
- セッションが存在しても AccessToken が空の場合、Authorization ヘッダーを設定しない
- API側でトークンなしとして扱われる

**選択肢B: エラーとして扱う**
- AccessToken が空の場合、502 Bad Gateway を返す
- セッションデータの不整合を明示的に検出

**決定: 選択肢A（スキップ）**

**理由:**
- セッション作成直後など、AccessToken がまだ設定されていない状態がありうる
- API側で適切な 401/403 を返すべきなので、BFF側でエラーにする必要はない
- ログに警告を出力して監視可能にする

```go
if sessionData.AccessToken != "" {
    request.Out.Header.Set("Authorization", "Bearer "+sessionData.AccessToken)
} else {
    slog.Warn("AccessToken is empty in session", "sessionId", sessionID.Value)
}
```

## Implementation Notes

### 変更箇所

**proxy/handler.go:**
1. `redis.GetSessionValue()` の戻り値を `sessionData` 変数に格納（行40）
2. `sessionData.AccessToken` が空でない場合、`Authorization` ヘッダーを設定
3. AccessToken が空の場合、警告ログを出力

### テストケース

**proxy/handler_test.go に追加すべきテスト:**
```go
// 正常系
- SessionData.AccessToken が設定されている場合、Authorization ヘッダーが付与される
- Authorization ヘッダーの形式が "Bearer <token>" である

// 異常系
- SessionData.AccessToken が空文字列の場合、Authorization ヘッダーが設定されない
- セッションがRedisに存在しない場合、Authorization ヘッダーが設定されない
```

### 実装ステップ

1. **proxy/handler.go の修正**
   - `redis.GetSessionValue()` の戻り値を変数に格納
   - Authorization ヘッダーを設定するロジックを追加

2. **テストの追加**
   - `proxy/handler_test.go` に上記テストケースを追加
   - Redisモックまたはテスト用Redis接続を使用

3. **動作確認**
   - `task bff:test` でテスト実行
   - 手動テスト: ブラウザでログイン後、API呼び出しのリクエストヘッダーを確認

## Consequences

**ポジティブ:**
- ✅ **機能の実現**: API側でユーザー認証情報（AccessToken）を利用可能に
- ✅ **セキュリティ向上**: BFF が認証を管理し、API側は検証のみに集中できる
- ✅ **既存動作の維持**: セッションローテーションなどの既存ロジックは変更なし
- ✅ **ログの充実**: AccessToken が空の場合に警告ログを出力

**ネガティブ:**
- ⚠️ **依存性の増加**: API側が Authorization ヘッダーを期待する前提になる
- ⚠️ **トークン漏洩リスク**: AccessToken がログに記録されないよう注意が必要（現在の実装では記録していない）

**リスク評価:**
- 🟢 **低リスク** — 既存のセッション検証ロジックに小さな追加のみ
- 🟢 **セキュリティリスク低** — AccessToken は既にRedisに保存されており、新たなリスクは導入しない

**代替案との比較:**

| 案 | メリット | デメリット |
|---------|---------|----------|
| **現状維持** | 変更なし | API側でユーザー認証情報が利用できない |
| **UserID/Email もヘッダーに追加** | API側で追加情報を利用可能 | ヘッダー数が増加、AccessToken から取得可能な情報を重複して送信 |
| **提案案（AccessToken のみ転送）** | シンプル、必要最小限 | API側で追加のユーザー情報が必要な場合、トークンから取得する必要がある |

**決定事項:**
- Redis から取得した `SessionData.AccessToken` を `Authorization: Bearer <token>` として API に転送
- AccessToken が空の場合、ヘッダーは設定せず警告ログを出力
- UserID、Email などの他の情報は、必要に応じてAPI側でトークンから取得
