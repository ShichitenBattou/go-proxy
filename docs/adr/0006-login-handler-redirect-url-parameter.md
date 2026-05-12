# ADR-0006: LoginHandler のリダイレクトURL パラメータ化

## Status
承認済み（Accepted） - 2026-04-16

## Context

### 現在の実装

`bff/auth/login_handler.go` において：

**問題点:**
1. `LoginRequest` 構造体（login_handler.go:14-16）が定義されているが、実際には使用されていない
2. `StateData.RedirectURL` に `*r.URL`（リクエストURL全体）を保存している（login_handler.go:39）
3. 型の不整合: `StateData.RedirectURL` は `url.URL` 型（値型）だが、`*r.URL` は `*url.URL` 型（ポインタ型）

**現在のコード:**
```go
type LoginRequest struct {
    RedirectURL string `json:"redirectUrl"`  // 未使用
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
    // ...
    stateData := redis.StateData{
        RedirectURL: *r.URL,  // リクエストURL全体を保存（意図と異なる可能性）
        CreatedAt:   time.Now().UTC(),
    }
    // ...
}
```

### 想定される本来の動作

1. ユーザーが保護されたページ（例: `https://auth.local/dashboard`）にアクセス
2. 未認証なので BFF が `/auth/login?redirect_url=/dashboard` にリダイレクト
3. ログイン成功後、元のページ（`/dashboard`）に戻る

**課題:**
- 現在の実装では、リダイレクト先URLをクエリパラメータから取得していない
- `LoginRequest` 構造体が定義されているが活用されていない

### StateData の定義

```go
// redis/redis.go:107-110
type StateData struct {
    RedirectURL url.URL   `json:"redirect_url"`
    CreatedAt   time.Time `json:"created_at"`
}
```

## Decision

### 1. クエリパラメータからリダイレクトURLを取得

**変更内容:**
- `redirect_url` クエリパラメータを取得
- パース・検証を行う
- `StateData.RedirectURL` に保存

**実装:**
```go
func LoginHandler(w http.ResponseWriter, r *http.Request) {
    cfg := setup.GetConfig()

    // クエリパラメータから redirect_url を取得
    redirectURLStr := r.URL.Query().Get("redirect_url")
    if redirectURLStr == "" {
        redirectURLStr = "/" // デフォルトはルートページ
    }

    // URL パース・検証
    redirectURL, err := url.Parse(redirectURLStr)
    if err != nil {
        slog.Error("Invalid redirect_url parameter", "error", err, "url", redirectURLStr)
        http.Error(w, "Invalid redirect URL", http.StatusBadRequest)
        return
    }

    // セキュリティ: 絶対URLの場合、ホストを検証（オープンリダイレクト防止）
    if redirectURL.IsAbs() {
        slog.Warn("Absolute URL in redirect_url, using path only", "url", redirectURLStr)
        redirectURL = &url.URL{Path: redirectURL.Path, RawQuery: redirectURL.RawQuery}
    }

    // ...
    stateData := redis.StateData{
        RedirectURL: *redirectURL,  // パース済みURLを保存
        CreatedAt:   time.Now().UTC(),
    }
    // ...
}
```

### 2. LoginRequest 構造体の扱い

**選択肢A: 削除する**
- クエリパラメータを直接 `r.URL.Query().Get()` で取得する場合、構造体は不要
- シンプルで直接的

**選択肢B: 保持して将来的に活用**
- 構造体ベースのバリデーションライブラリ（例: go-playground/validator）を使う場合に有用
- 現時点では使用しないが、将来的な拡張性を考慮

**決定: 選択肢A（削除）**

**理由:**
- 現在のシンプルな要件（単一パラメータ取得）には構造体は過剰
- YAGNI原則（You Aren't Gonna Need It）— 必要になったら追加すれば良い
- 未使用コードを残すとメンテナンスコストが増加

### 3. セキュリティ対策

**オープンリダイレクト脆弱性の防止:**
- 絶対URLは拒否または相対パスのみ抽出
- 許可されたドメインのホワイトリストチェック（オプション）

**実装方針:**
```go
// 絶対URLの場合、パス部分のみを使用
if redirectURL.IsAbs() {
    redirectURL = &url.URL{Path: redirectURL.Path, RawQuery: redirectURL.RawQuery}
}
```

**代替案（より厳格）:**
```go
// 絶対URLは完全拒否
if redirectURL.IsAbs() {
    http.Error(w, "Only relative URLs are allowed", http.StatusBadRequest)
    return
}
```

**決定: 前者（パス抽出）**
- ユーザビリティ: エラーではなく自動修正
- ログに警告を記録して監視可能

## Implementation Notes

### 変更箇所

**auth/login_handler.go:**
1. `LoginRequest` 構造体を削除（行14-16）
2. クエリパラメータ取得ロジックを追加
3. URL検証ロジックを追加
4. `StateData` への設定を `*redirectURL` に変更

### テストケース

**login_handler_test.go に追加すべきテスト:**
```go
// 正常系
- redirect_url パラメータありの場合
- redirect_url パラメータなしの場合（デフォルト"/"）

// 異常系
- 不正なURL形式の場合（400 Bad Request）
- 絶対URLの場合（パスのみ抽出、警告ログ）

// セキュリティ
- オープンリダイレクト試行（https://evil.com → path のみ抽出）
```

### 実装ステップ

1. **login_handler.go の修正**
   - `LoginRequest` 構造体を削除
   - クエリパラメータ取得ロジックを追加
   - URL検証ロジックを追加

2. **テストの追加**
   - `login_handler_test.go` に上記テストケースを追加
   - Redisモックまたはテスト用Redis接続を使用

3. **動作確認**
   - `task bff:test` でテスト実行
   - 手動テスト: `curl -i 'http://localhost:8080/auth/login?redirect_url=/dashboard'`

## Consequences

**ポジティブ:**
- ✅ **機能の実現**: リダイレクトURLをクエリパラメータで指定可能に
- ✅ **セキュリティ向上**: オープンリダイレクト脆弱性の防止
- ✅ **コードの明確化**: 未使用の `LoginRequest` 構造体を削除し、意図が明確に
- ✅ **型整合性**: `url.URL` と `*url.URL` の不整合を解消

**ネガティブ:**
- ⚠️ **後方互換性**: 現在 `redirect_url` パラメータを使用していないため、影響なし
- ⚠️ **複雑性の増加**: URL検証ロジックが追加されるが、セキュリティのために必要

**リスク評価:**
- 🟢 **低リスク** — 新機能追加であり、既存動作は維持
- 🟡 **中セキュリティリスク** — オープンリダイレクト対策が不十分な場合、脆弱性が残る（対策実装済み）

**代替案との比較:**

| 案 | メリット | デメリット |
|---------|---------|----------|
| **現状維持** | 変更なし | `LoginRequest` が無駄、リダイレクト機能が不完全 |
| **LoginRequest 構造体を活用** | 構造化されたバリデーション可能 | 現在の要件には過剰、複雑性増加 |
| **提案案（クエリパラメータ直接取得）** | シンプル、YAGNI原則準拠 | 将来的にバリデーションライブラリ導入時に再構造化が必要 |

**決定事項:**
- クエリパラメータから `redirect_url` を取得
- `LoginRequest` 構造体は削除
- オープンリダイレクト対策を実装（絶対URLはパスのみ抽出）
