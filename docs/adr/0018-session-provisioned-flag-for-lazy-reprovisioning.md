# ADR-0018: セッションフラグによる遅延再プロビジョニング

## Status
承認済み（Accepted） - 2026-05-09

## Context

### 問題: JIT プロビジョニングの失敗でユーザーが API を使えない状態になる

ADR-0017 で採用した JIT プロビジョニング（`/auth/callback` 内での `POST /users` 呼び出し）はソフトフェイルで設計されている。API が一時的に停止している場合、ログインは成功するがユーザーが `users` テーブルに存在しない状態になる。

この状態でユーザーが API を呼び出すと、API 側の `get_current_user` 依存で 401 または「User not registered.」エラーが返り続ける。再ログインするまで復旧しない。

### 検討した代替案

**案A: API のエラーレスポンスを検出してプロキシ層でリトライする**

`proxy/handler.go` の `ModifyResponse` でレスポンスボディを解析し、特定のエラー（例: `"User not registered."`）を検出したら `ProvisionUser` を呼んで元のリクエストを再送する方式。

懸念点:
- **リクエストボディの再送不可**: `httputil.ReverseProxy` はリクエストボディを消費するため、POST/PUT 等の再送にはボディの事前バッファが必要。大きなアップロード時にメモリが圧迫される。
- **レスポンスボディのバッファが必要**: エラー種別を判定するにはボディを読み切る必要があり、ストリーミングが壊れる。
- **無限ループのリスク**: 再送後も同じエラーが返る場合（API の反映タイミング等）に際限なくループしうる。リトライ上限管理が必要で複雑さが増す。
- **エラー文字列への依存**: API のエラーメッセージ変更で BFF が壊れる。ステータスコードは他の原因でも返るため誤検知のリスクがある。
- **責務の混在**: 透過プロキシ層にビジネスロジック（ユーザー登録判定）が入り込み、Vertical Slice Architecture の方針に反する。

**案B: セッションに `Provisioned` フラグを持たせる（採用案）**

`SessionData` に `Provisioned bool` フィールドを追加し、プロビジョニング成功時に `true` にする。プロキシ層はリクエスト転送前にフラグを確認し、`false` であれば `ProvisionUser` を呼んでからリクエストを転送する。

## Decision

**案B（セッションフラグ方式）** を採用する。

### 採用理由

- リクエストボディを読む必要がなく、再送問題が発生しない。
- レスポンスボディを解析しないためストリーミングへの影響がない。
- 転送前にプロビジョニングを完了するため、無限ループが構造上起きない。
- API のエラーフォーマットに依存しない。
- `proxy/` の変更量が最小（`rewrite` 関数の先頭に数行追加するだけ）。

### 変更後のフロー

```
/auth/callback（ログイン時）
  → JIT プロビジョニング成功 → SessionData.Provisioned = true
  → JIT プロビジョニング失敗 → SessionData.Provisioned = false（ソフトフェイルは維持）

/api/* へのリクエスト時（proxy/handler.go の rewrite）
  → SessionData.Provisioned == true → そのまま転送
  → SessionData.Provisioned == false
      → ProvisionUser を呼び出す
      → 成功 → SessionData.Provisioned = true に更新して転送
      → 失敗 → ログのみ（転送は継続、API 側で 401 が返る）
```

### 変更概要

#### BFF: `redis/redis.go`

`SessionData` に `Provisioned` フィールドを追加する:

```go
type SessionData struct {
    UserID       string `json:"user_id"`
    Email        string `json:"email"`
    Name         string `json:"name"`
    IDToken      string `json:"id_token"`
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    Provisioned  bool   `json:"provisioned"` // [NEW]
}
```

#### BFF: `auth/callback_handler.go`（`completeCallback`）

プロビジョニング結果に応じて `Provisioned` フラグをセットする:

```go
provisioned := true
if err := provisionFn(r.Context(), tokens.APIAccessToken, cfg.ProxyTarget); err != nil {
    slog.Warn("JIT provisioning failed; will retry on next request",
        "sub", claims.Sub,
        "error", err,
    )
    provisioned = false
}

sessionData := redis.SessionData{
    // ... 既存フィールド
    Provisioned: provisioned,
}
```

#### BFF: `proxy/handler.go`（`rewrite`）

転送前にフラグを確認し、未プロビジョニングであれば再試行する:

```go
if !sessionData.Provisioned {
    slog.Info("Session not provisioned, attempting provisioning", "sessionId", sessionID.Value)
    if err := auth.ProvisionUser(request.In.Context(), sessionData.AccessToken, forwardHost); err != nil {
        slog.Warn("Re-provisioning failed, forwarding request anyway", "error", err)
    } else {
        sessionData.Provisioned = true
        if err := redis.UpdateSession(sessionID.Value, sessionData); err != nil {
            slog.Warn("Failed to update session provisioned flag", "error", err)
        }
    }
}
```

#### BFF: `redis/redis.go`

セッションデータを更新する `UpdateSession` 関数を追加する（既存の TTL を維持した上書き保存）:

```go
// UpdateSession は既存セッションの内容を上書きする。
// TTL はセッション作成時の値を引き継ぐため、ここでは再設定しない。
func UpdateSession(sessionID string, data SessionData) error {
    // SetSession と同一ロジック（TTL 付き SETEX）
}
```

## Consequences

**ポジティブ:**
- API が一時停止から回復した後、次のリクエストで自動的にプロビジョニングが完了する（再ログイン不要）。
- ストリーミングレスポンスへの影響なし。
- リクエストボディのバッファが不要（メモリ効率を維持）。
- API のエラーフォーマット変更に影響を受けない。
- プロキシ層の変更が最小限で済む。

**ネガティブ:**
- `proxy/` が `auth.ProvisionUser` を呼び出すことで、スライス間に依存が生じる。`auth/provision.go` の関数を `redis/` や新設の `provisioning/` スライスに移動するリファクタリングが将来必要になりうる。
- `redis.UpdateSession` を新規追加する必要がある（実装コスト小）。
- プロビジョニングが未完了のままのセッションが Redis に残存しうる（TTL 期限切れで自然消滅する）。

## Implementation Notes

- `ProvisionUser` の呼び出し先は既存の `PROXY_TARGET`（`forwardHost` 引数）をそのまま使用する。
- 再プロビジョニング失敗時はログのみとし、リクエスト転送は継続する（API 側で 401 が返るのはユーザーへのフィードバックとして許容）。
- `UpdateSession` の TTL は `SetSession` と同じ `SESSION_TTL` を適用する（既存セッションの有効期限を延長しないよう注意）。
- `proxy/handler.go` の `rewrite` 関数内で呼び出すため、HTTP クライアントのタイムアウト（`provisionHTTPClient` の 10 秒）が rewrite のレイテンシに直接影響する点に注意。
- 既存の `Provisioned` フィールドがない旧セッションは `false` のゼロ値になるため、既存セッションに対して一度再プロビジョニングが実行される（`CreateUserInteractor` がべき等なので問題なし）。
