# ADR-0008: ログインフローのセキュリティ強化

## Status
提案中（Proposed） - 2026-04-18

## Context

### 現在の実装

`bff/auth/login_handler.go` と `bff/auth/callback_handler.go` において、OIDC 認証フローは実装されているが、いくつかの重大なセキュリティ上の問題が存在する。

### 問題点

#### 1. **State token に TTL が設定されていない（Critical）**

**redis/redis.go:123:**
```go
func SetState(state uuid.UUID, stateData StateData) error {
    // ...
    err = rdb.Set(ctx, key, content, 0).Err()  // TTL = 0 = 無期限
    // ...
}
```

**リスク:**
- 🔴 古い state token が永久に Redis に残り、メモリリーク
- 🔴 リプレイ攻撃のリスク（古い state を再利用して認証バイパス）
- 🔴 CSRF 保護の実効性低下

**影響範囲:**
- すべての `/auth/login` → `/auth/callback` フロー

#### 2. **State token の期限検証がない（Critical）**

**callback_handler.go:37-42:**
```go
stateData, err := redis.GetStateValue(stateUUID)
if err != nil {
    slog.Error("Failed to get state from Redis", "state", stateUUID, "error", err)
    http.Error(w, "Invalid state parameter", http.StatusForbidden)
    return
}
// ⚠️ stateData.CreatedAt のチェックがない
```

**現状:**
- `StateData.CreatedAt` フィールドは存在する（redis/redis.go:109）
- しかし、CallbackHandler で時刻チェックを行っていない
- Redis に state が存在すれば、何時間・何日前のものでも受理される

**リスク:**
- 🔴 古い state token を使った攻撃が可能
- 🔴 CSRF 保護の時間的制約がない

#### 3. **Open redirect 対策が不完全（Medium）**

**login_handler.go:33-37:**
```go
// Security: Prevent open redirect - if absolute URL, extract path only
if redirectURL.IsAbs() {
    slog.Warn("Absolute URL in redirect_url, using path only", "url", redirectURLStr)
    redirectURL = &url.URL{Path: redirectURL.Path, RawQuery: redirectURL.RawQuery}
}
```

**現在の対策:**
- 絶対 URL (`https://evil.com`) の場合、パス部分のみ抽出

**脆弱性:**
- `//evil.com` (プロトコル相対 URL) — `IsAbs()` は `false` を返すため、そのまま通過
- `///evil.com` (スラッシュ3つ) — 同様に通過
- `/%0D%0A` (改行文字) — HTTP ヘッダーインジェクションの可能性
- 任意のパスへのリダイレクトが可能 — `/admin`, `/api/internal` など

**リスク:**
- 🟡 フィッシング攻撃の足がかり
- 🟡 意図しないページへの誘導

#### 4. **本番コードに不要な接続テスト（Minor）**

**login_handler.go:39-46:**
```go
// Test Keycloak connectivity
client := utils.GetInternalHTTPClient()
res, err := client.Get(cfg.OIDCProviderURL)
if err != nil {
    slog.Debug("Error connecting to Keycloak", "error", err)
} else {
    slog.Debug("Successfully connected to Keycloak", "statusCode", res.StatusCode, "content", res.Body)
}
```

**問題:**
- 毎回のログインリクエストで Keycloak に余分な HTTP 接続
- パフォーマンス影響（わずかだが不要）
- デバッグ用コードが本番に残っている

## Decision

### 1. State token に TTL を設定（Critical）

**変更内容:**
- `SetState()` で Redis TTL を **5分** に設定
- OAuth2 の標準的な state 有効期間に準拠

**実装:**
```go
func SetState(state uuid.UUID, stateData StateData) error {
    cfg := setup.GetConfig()
    rdb := createClient()
    defer rdb.Close()

    content, err := json.Marshal(stateData)
    if err != nil {
        return err
    }

    key := cfg.StateKeyPrefix + state.String()
    err = rdb.Set(ctx, key, content, cfg.StateTTL).Err()  // 👈 変更: TTL を設定
    if err != nil {
        return err
    }
    slog.Info("State stored in Redis", "key", key, "ttl", cfg.StateTTL)
    return nil
}
```

**環境変数の追加:**
- `STATE_TTL` — デフォルト `5m` (5分)
- `setup/config.go` に追加

### 2. State token の期限検証を追加（Critical）

**変更内容:**
- CallbackHandler で `StateData.CreatedAt` をチェック
- 設定された TTL を超えている場合は 403 Forbidden

**実装:**
```go
func CallbackHandler(w http.ResponseWriter, r *http.Request) {
    cfg := setup.GetConfig()
    // ...

    stateData, err := redis.GetStateValue(stateUUID)
    if err != nil {
        slog.Error("Failed to get state from Redis", "state", stateUUID, "error", err)
        http.Error(w, "Invalid state parameter", http.StatusForbidden)
        return
    }

    // 👈 追加: State token の期限検証
    if time.Since(stateData.CreatedAt) > cfg.StateTTL {
        slog.Error("State token expired", "state", stateUUID, "createdAt", stateData.CreatedAt, "age", time.Since(stateData.CreatedAt))
        redis.DeleteState(stateUUID)  // 期限切れ state を削除
        http.Error(w, "State expired", http.StatusForbidden)
        return
    }

    // 既存の Delete state token
    if err := redis.DeleteState(stateUUID); err != nil {
        slog.Error("Failed to delete state from Redis", "error", err)
    }
    // ...
}
```

**設計ノート:**
- Redis TTL と CreatedAt チェックの二重検証（Defense in Depth）
- Redis TTL で自動削除、CreatedAt でアプリケーション層検証
- Redis クロックと BFF クロックのズレに対応

### 3. Open redirect 対策の強化（Medium）

**変更内容:**
- リダイレクト URL のホワイトリスト検証を追加
- プロトコル相対 URL、改行文字のチェック

**実装:**

**3-1. 環境変数でホワイトリストを設定:**

```bash
# .env
ALLOWED_REDIRECT_PATH_PATTERN=^/(dashboard|profile|settings)(/.*)?$
```

**3-2. setup/config.go に追加:**

```go
type Config struct {
    // ...
    AllowedRedirectPathPattern string `env:"ALLOWED_REDIRECT_PATH_PATTERN"`
}

// Validate validates the configuration and compiles regex patterns
func (cfg *Config) Validate() error {
    // Compile redirect path pattern to detect errors early
    if cfg.AllowedRedirectPathPattern != "" {
        _, err := regexp.Compile(cfg.AllowedRedirectPathPattern)
        if err != nil {
            return fmt.Errorf("invalid ALLOWED_REDIRECT_PATH_PATTERN: %w", err)
        }
    }
    return nil
}
```

**3-3. login_handler.go で検証:**

```go
func LoginHandler(w http.ResponseWriter, r *http.Request) {
    cfg := setup.GetConfig()

    // Get redirect_url from query parameters
    redirectURLStr := r.URL.Query().Get("redirect_url")
    if redirectURLStr == "" {
        redirectURLStr = "/"
    }

    // 👈 追加: プロトコル相対URLのチェック
    if strings.HasPrefix(redirectURLStr, "//") {
        slog.Error("Protocol-relative URL detected", "url", redirectURLStr)
        http.Error(w, "Invalid redirect URL", http.StatusBadRequest)
        return
    }

    // 👈 追加: 改行文字のチェック（ヘッダーインジェクション対策）
    if strings.ContainsAny(redirectURLStr, "\r\n") {
        slog.Error("Newline character detected in redirect URL", "url", redirectURLStr)
        http.Error(w, "Invalid redirect URL", http.StatusBadRequest)
        return
    }

    // Parse and validate redirect URL
    redirectURL, err := url.Parse(redirectURLStr)
    if err != nil {
        slog.Error("Invalid redirect_url parameter", "error", err, "url", redirectURLStr)
        http.Error(w, "Invalid redirect URL", http.StatusBadRequest)
        return
    }

    // Security: Prevent open redirect
    if redirectURL.IsAbs() {
        slog.Warn("Absolute URL in redirect_url, using path only", "url", redirectURLStr)
        redirectURL = &url.URL{Path: redirectURL.Path, RawQuery: redirectURL.RawQuery}
    }

    // 👈 追加: パスのホワイトリスト検証
    if !isAllowedRedirectPath(redirectURL.Path, cfg) {
        slog.Error("Redirect path not in whitelist", "path", redirectURL.Path)
        http.Error(w, "Redirect URL not allowed", http.StatusForbidden)
        return
    }

    // ...
}

// 👈 追加: ホワイトリスト検証関数
func isAllowedRedirectPath(path string, cfg *setup.Config) bool {
    // パターンが設定されていない場合は全て許可（後方互換性）
    if cfg.AllowedRedirectPathPattern == "" {
        return true
    }

    // 正規表現でマッチング
    matched, err := regexp.MatchString(cfg.AllowedRedirectPathPattern, path)
    if err != nil {
        // 起動時バリデーションで検出されるはずだが、念のためログ出力
        slog.Error("Invalid redirect path pattern", "pattern", cfg.AllowedRedirectPathPattern, "error", err)
        return false
    }
    return matched
}
```

**設定例:**

| ユースケース | 正規表現パターン | 説明 |
|--------------|------------------|------|
| **ルートのみ許可** | `^/$` | `/` のみ許可 |
| **特定のパスのみ許可** | `^/(dashboard\|profile\|settings)$` | `/dashboard`, `/profile`, `/settings` のみ |
| **サブパスも許可** | `^/(dashboard\|profile\|settings)(/.*)?$` | `/dashboard/users` なども許可 |
| **前方一致で許可** | `^/app/.*$` | `/app/` 以下すべて許可 |
| **複雑な条件** | `^/(public\|user/[^/]+/dashboard)$` | `/public` と `/user/*/dashboard` を許可 |
| **すべて許可（非推奨）** | `^/.*$` または設定なし | セキュリティリスクあり |

### 4. 不要な接続テストの削除（Minor）

**変更内容:**
- `LoginHandler` から Keycloak 接続テストコードを削除
- 必要であれば、起動時のヘルスチェックや専用エンドポイントで実装

**削除するコード:**
```go
// 削除対象
client := utils.GetInternalHTTPClient()
res, err := client.Get(cfg.OIDCProviderURL)
if err != nil {
    slog.Debug("Error connecting to Keycloak", "error", err)
} else {
    slog.Debug("Successfully connected to Keycloak", "statusCode", res.StatusCode, "content", res.Body)
}
```

## Implementation Notes

### 変更箇所

**1. setup/config.go:**
- `StateTTL time.Duration` フィールドを追加（デフォルト: `5m`）
- `AllowedRedirectPathPattern string` フィールドを追加
- `Validate() error` メソッドを追加（起動時に正規表現をコンパイル検証）

**2. redis/redis.go:**
- `SetState()` — TTL を `cfg.StateTTL` に変更（行123）
- ログメッセージに TTL を追加

**3. auth/callback_handler.go:**
- State token 期限検証ロジックを追加（`GetStateValue()` 直後）
- 期限切れの場合、`DeleteState()` してから 403 を返す

**4. auth/login_handler.go:**
- プロトコル相対 URL チェックを追加
- 改行文字チェックを追加
- ホワイトリスト検証ロジックを追加（`isAllowedRedirectPath()` 関数）
- Keycloak 接続テストコードを削除（行39-46）

**5. auth/validation.go (新規作成):**
- `isAllowedRedirectPath()` 関数を実装
- リダイレクト URL 検証ロジックを集約

### テストケース

**redis/redis_test.go:**
```go
// SetState のTTL検証
- State保存時にTTLが設定されること
- 設定したTTL後にRedisから自動削除されること
```

**auth/callback_handler_test.go:**
```go
// State期限検証
- 正常系: 5分以内のstateは受理される
- 異常系: 5分を超えたstateは403 Forbiddenを返す
- 異常系: 期限切れstateはRedisから削除される
```

**auth/login_handler_test.go:**
```go
// Open redirect対策
- 正常系: 許可されたパスへのリダイレクトは成功
- 異常系: //evil.com は400 Bad Request
- 異常系: ///evil.com は400 Bad Request
- 異常系: /%0D%0A は400 Bad Request
- 異常系: ホワイトリストにないパスは403 Forbidden
- 正常系: ホワイトリスト設定がない場合は全パス許可（後方互換）

// 接続テスト削除
- Keycloak接続テストが実行されないこと（パフォーマンス確認）
```

**auth/validation_test.go (新規):**
```go
// isAllowedRedirectPath のユニットテスト
- 正規表現パターン: 完全一致
- 正規表現パターン: 前方一致（/app/.* → /app/dashboard）
- 正規表現パターン: サブパス含む ((/.*)?$ パターン)
- 正規表現パターン: マッチ失敗
- 正規表現パターン: 不正な正規表現（エラーハンドリング）
- パターン未設定: 全て許可（後方互換性）
```

**setup/config_test.go (追加):**
```go
// Config.Validate() のテスト
- 正常系: 有効な正規表現パターン
- 異常系: 不正な正規表現パターン（エラーを返す）
- 正常系: パターン未設定（エラーなし）
```

### 実装ステップ

1. **setup/config.go の修正**
   - `StateTTL`, `AllowedRedirectPathPattern` フィールドを追加
   - `Validate()` メソッドを追加（正規表現のコンパイル検証）
   - `.env.example` を更新（充実した正規表現例とコメントを記載）

2. **redis/redis.go の修正**
   - `SetState()` で TTL を設定

3. **auth/callback_handler.go の修正**
   - State 期限検証ロジックを追加

4. **auth/validation.go の作成**
   - `isAllowedRedirectPath()` 関数を実装
   - テストファイル `auth/validation_test.go` も作成

5. **auth/login_handler.go の修正**
   - プロトコル相対 URL・改行文字チェックを追加
   - ホワイトリスト検証を追加
   - Keycloak 接続テストを削除

6. **テストの追加**
   - 各ファイルのテストケースを追加

7. **動作確認**
   - `task bff:test` でテスト実行
   - 手動テスト:
     - 正常なログインフロー
     - 古い state token（5分後）での callback アクセス → 403
     - Open redirect 試行 → 400/403

## Consequences

### ポジティブ

- ✅ **セキュリティ大幅向上**: CSRF 保護が時間的制約を持つ
- ✅ **メモリリーク防止**: State token が自動的に削除される
- ✅ **Open redirect 対策強化**: プロトコル相対 URL、改行文字、ホワイトリスト検証
- ✅ **柔軟性**: 環境変数でホワイトリストとTTLを設定可能
- ✅ **パフォーマンス改善**: 不要な Keycloak 接続テストを削除
- ✅ **監査性向上**: ログに TTL、期限切れ、検証失敗が記録される

### ネガティブ

- ⚠️ **設定の複雑化**: 環境変数が2つ増加（`STATE_TTL`, `ALLOWED_REDIRECT_PATH_PATTERN`）
- ⚠️ **正規表現の学習コスト**: 正規表現に不慣れなユーザーには設定ミスのリスク（`.env.example` で緩和）
- ⚠️ **後方互換性**: ホワイトリスト設定がない場合は全パス許可（段階的導入可能）
- ⚠️ **運用コスト**: ホワイトリストのメンテナンスが必要（新しいページ追加時）
- ⚠️ **起動時エラー**: 不正な正規表現の場合、起動に失敗する（設定ミスの早期検出になる）

### リスク評価

- 🟢 **セキュリティリスク: 大幅改善**
  - State token リプレイ攻撃: 不可能に
  - Open redirect: 大幅に困難に
  - CSRF: より強固な保護
- 🟡 **実装リスク: 低〜中**
  - テストカバレッジが必要（特に TTL と期限検証）
  - 時刻同期の問題（BFF と Redis のクロックズレ）に注意

### 代替案との比較

| 案 | メリット | デメリット |
|---------|---------|----------|
| **現状維持** | 変更なし | 🔴 重大なセキュリティ脆弱性が残る |
| **TTL のみ設定** | シンプル | State 期限検証がアプリケーション層にない（クロックズレに弱い） |
| **期限検証のみ追加** | Redis TTL 不要 | メモリリーク、Redis 容量圧迫 |
| **提案案（TTL + 期限検証 + ホワイトリスト）** | 多層防御、柔軟性 | 実装量が多い、設定が増える |
| **ホワイトリストを完全必須化** | 最も安全 | 既存環境への影響大、段階的導入不可 |

### 決定事項

1. **State token に 5分の TTL を設定** — Redis レベルでの自動削除
2. **CallbackHandler で CreatedAt を検証** — アプリケーションレベルでの二重チェック
3. **リダイレクト URL のホワイトリスト検証** — 環境変数で柔軟に設定
4. **プロトコル相対 URL・改行文字のチェック** — 既知の攻撃パターンをブロック
5. **不要な接続テストを削除** — パフォーマンス改善
6. **ホワイトリスト未設定時は全パス許可** — 後方互換性と段階的導入

### セキュリティ基準

**OAuth2 / OIDC ベストプラクティス準拠:**
- ✅ State token の時間的制約（RFC 6749 Section 10.12）
- ✅ CSRF 保護（RFC 6749 Section 10.12）
- ✅ Open Redirect 対策（OWASP Top 10）
- ✅ 多層防御（Defense in Depth）

**推奨される本番環境設定:**
```bash
# .env.example (推奨設定とコメント)

# State token TTL (default: 5m)
STATE_TTL=5m

# Redirect URL whitelist (regex pattern)
# Examples:
#   ^/$                                      # Root only
#   ^/(dashboard|profile|settings)$          # Specific paths only
#   ^/(dashboard|profile|settings)(/.*)?$    # Paths with optional subpaths
#   ^/app/.*$                                # All paths under /app
#   ^/(public|user/[^/]+/dashboard)$         # Complex conditions
#   ^/.*$                                    # All paths (NOT RECOMMENDED)
#
# If not set, all paths are allowed (backward compatibility)
# For production, it is HIGHLY RECOMMENDED to set a restrictive pattern
ALLOWED_REDIRECT_PATH_PATTERN=^/(dashboard|profile|settings)(/.*)?$
```

**本番環境での設定例:**
```bash
# .env (本番環境)
STATE_TTL=5m
ALLOWED_REDIRECT_PATH_PATTERN=^/(dashboard|profile|settings|app)(/.*)?$
```
