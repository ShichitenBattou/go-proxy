# ADR-0005: auth/handler.go のハンドラー分割

## Status
承認済み（Accepted） - 2026-04-16

## Context

現在、`bff/auth/handler.go` には2つのエンドポイントハンドラーが同居しています（auth/handler.go:45-206）：

### 現在の構成

```
auth/
├── handler.go             # LoginHandler + CallbackHandler (207行)
├── me_handler.go          # MeHandler（既に分割済み）
├── logout_handler.go      # LogoutHandler（既に分割済み）
└── (テストファイル)
```

**handler.go の内容:**
1. `getOAuth2Config()` 関数（行18-39）— OAuth2設定を取得する共通関数
2. `LoginRequest` 構造体（行41-43）— 未使用の構造体
3. `LoginHandler` 関数（行45-76）— `/auth/login` エンドポイント
4. `IDTokenClaims` 構造体（行78-84）— ID Token のクレーム定義
5. `CallbackHandler` 関数（行86-206）— `/auth/callback` エンドポイント

### 課題

1. **一貫性の欠如**: `/auth/me` と `/auth/logout` は既に独立ファイルに分割済みだが、`/auth/login` と `/auth/callback` だけが `handler.go` に残っている
2. **ファイルサイズ**: handler.go が207行と肥大化しており、可読性が低下
3. **Vertical Slice Architecture との整合性**: 各エンドポイントを独立したスライスとして管理すべき（CLAUDE.md の方針）

### 既存パターン

`me_handler.go` と `logout_handler.go` は既に以下のパターンで分割されています：
- ファイル名: `<エンドポイント名>_handler.go`
- 1ファイル1ハンドラー（関連する構造体・ヘルパー関数を含む）
- パッケージ: すべて `package auth`

## Decision

### 1. ハンドラーファイルの分割

**Before:**
```
auth/
└── handler.go  (LoginHandler + CallbackHandler + 共通関数)
```

**After:**
```
auth/
├── login_handler.go      # LoginHandler
├── callback_handler.go   # CallbackHandler
├── oauth2.go             # getOAuth2Config() 共通関数
└── (既存の me_handler.go, logout_handler.go はそのまま)
```

### 2. ファイル内容の配置

#### `login_handler.go`
- `LoginHandler` 関数
- `LoginRequest` 構造体（現在未使用だが、将来的に使う可能性を考慮して残す）

#### `callback_handler.go`
- `CallbackHandler` 関数
- `IDTokenClaims` 構造体（CallbackHandler でのみ使用）

#### `oauth2.go`（新規作成）
- `getOAuth2Config()` 関数（LoginHandler と CallbackHandler 両方で使用される共通関数）

**理由:**
- 複数のハンドラーから参照される共通関数は、専用ファイルに分離することで依存関係を明確化
- oauth2.go という名前で OAuth2 関連の共通処理をまとめる（将来的にトークンリフレッシュなどの機能追加も想定）

### 3. main.go への影響

**影響なし** — すべて同じ `auth` パッケージ内の関数であるため、インポートパスは変わりません：

```go
// main.go (変更不要)
import "bff/auth"

r.Route("/auth", func(r chi.Router) {
    r.Get("/login", auth.LoginHandler)      // login_handler.go から提供
    r.Get("/callback", auth.CallbackHandler) // callback_handler.go から提供
    r.Get("/me", auth.MeHandler)
    r.Post("/logout", auth.LogoutHandler)
})
```

### 4. パッケージ構造の方針

Vertical Slice Architecture に従い、エンドポイント単位でファイルを分割：
- **1エンドポイント = 1ファイル** （ハンドラー + 関連構造体）
- **共通ロジック** は明示的に分離（例: oauth2.go）

## Implementation Notes

### 実装ステップ

1. **oauth2.go の作成**
   - `getOAuth2Config()` 関数を handler.go から移動
   - 必要なインポート（context, slog, oidc, oauth2, utils, setup）を追加

2. **login_handler.go の作成**
   - `LoginHandler` 関数を handler.go から移動
   - `LoginRequest` 構造体を移動
   - 必要なインポートを追加（oauth2.go の関数を利用）

3. **callback_handler.go の作成**
   - `CallbackHandler` 関数を handler.go から移動
   - `IDTokenClaims` 構造体を移動
   - 必要なインポートを追加

4. **handler.go の削除**
   - 上記3ファイルへの移動完了後、handler.go を削除

5. **テストファイルの確認**
   - `handler_test.go` が存在する場合、対応するテストファイル名に変更
   - 現在の handler_test.go の内容を確認し、login_handler_test.go と callback_handler_test.go に分割

### テストへの影響

既存のテスト（`auth/handler_test.go`）は分割が必要：
- `login_handler_test.go` — LoginHandler のテスト
- `callback_handler_test.go` — CallbackHandler のテスト
- `oauth2_test.go` — getOAuth2Config のテスト（必要に応じて）

**注意:** テストコードが既に存在する場合、同様に分割する。

### ビルドへの影響

**影響なし** — Go のパッケージシステムでは、同一パッケージ内の複数ファイルは自動的に統合されます。

## Consequences

**ポジティブ:**
- ✅ **一貫性の向上**: 全エンドポイントが独立ファイルで管理され、me_handler.go/logout_handler.go と統一されたパターンになる
- ✅ **可読性の向上**: 1ファイルあたりの行数が減り、各エンドポイントの実装が見やすくなる
- ✅ **Vertical Slice Architecture の徹底**: エンドポイント単位でコードをまとめる方針が明確化
- ✅ **責務の明確化**: 共通ロジック（OAuth2設定）が oauth2.go として分離され、依存関係が可視化
- ✅ **保守性の向上**: 各エンドポイントの変更時、関連するファイルのみを編集すれば良い

**ネガティブ:**
- ⚠️ **ファイル数の増加**: handler.go 1つ → 3ファイル（login_handler.go, callback_handler.go, oauth2.go）
  - **影響**: 小規模プロジェクトではオーバーエンジニアリングに見える可能性があるが、既に me_handler.go/logout_handler.go が分離されているため一貫性を保つべき
- ⚠️ **初回学習コスト**: 新規開発者がファイル構造を理解する際、複数ファイルを見る必要がある
  - **影響**: ファイル名が明確（エンドポイント名と一致）なので、実際の影響は最小限

**リスク評価:**
- 🟢 **低リスク** — パッケージ内のファイル再配置のみで、外部APIは変わらない
- 🟢 **高メリット** — コードの一貫性と保守性が向上

**代替案との比較:**

| 案 | メリット | デメリット |
|---------|---------|----------|
| **現状維持** | ファイル数が少ない | me_handler.go/logout_handler.go との一貫性欠如、handler.go が肥大化 |
| **login_handler.go + callback_handler.go に分割（oauth2.go なし）** | ファイル数が2つで済む | getOAuth2Config() の重複配置が必要、共通ロジックの管理が曖昧 |
| **提案案（3ファイル分割）** | 一貫性・責務明確・保守性向上 | ファイル数が最も多い（許容範囲） |

**決定事項:**
- handler.go を3ファイルに分割（login_handler.go, callback_handler.go, oauth2.go）
- 既存の me_handler.go/logout_handler.go パターンに統一
- main.go は変更不要（同一パッケージ内の移動のため）
