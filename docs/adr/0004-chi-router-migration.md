# ADR-0004: Chi ルーターへの移行

## Status
提案中（Proposed） - 2026-04-16

## Context

現在、BFF は標準ライブラリの `net/http` でルーティングを実装しています（`bff/main.go:26-34`）。

### 現在の実装

```go
http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
    slog.Info("Health check", "url", r.URL.String(), "requestedHost", r.Host, "ip", r.RemoteAddr)
    w.WriteHeader(http.StatusOK)
})
http.Handle("/", proxy.NewHandler(cfg.ProxyTarget))
http.HandleFunc("/auth/login", auth.LoginHandler)
http.HandleFunc("/auth/callback", auth.CallbackHandler)
http.HandleFunc("/auth/me", auth.MeHandler)
http.HandleFunc("/auth/logout", auth.LogoutHandler)
```

### 課題

1. **ミドルウェア管理の煩雑さ**: 標準ライブラリではミドルウェアチェーンが書きにくい
2. **ルーティング制御の限界**: パスパラメータやサブルーター機能が弱い
3. **可読性**: ルート定義が散在しやすい
4. **拡張性**: 将来的に API バージョニングやグループ化が必要になった際の対応

### Chi の選定理由

[Chi](https://github.com/go-chi/chi) は以下の特徴を持つ軽量で高速な Go ルーターです：

- ✅ 標準ライブラリの `net/http` と互換性がある（既存ハンドラーをそのまま使える）
- ✅ ミドルウェアサポートが充実
- ✅ サブルーターでルートをグループ化可能
- ✅ パスパラメータ対応（例: `/users/{userID}`）
- ✅ Go コミュニティで広く使われている（信頼性・メンテナンス性）
- ✅ 最小限の依存関係（標準ライブラリベース）

## Decision

### 1. Chi ルーターの導入

**依存関係追加**

```bash
go get -u github.com/go-chi/chi/v5
```

### 2. main.go のルーティング定義変更

**Before (net/http):**
```go
http.HandleFunc("/health", healthHandler)
http.Handle("/", proxy.NewHandler(cfg.ProxyTarget))
http.HandleFunc("/auth/login", auth.LoginHandler)
// ...
http.ListenAndServe(cfg.BFFListenAddr, nil)
```

**After (Chi):**
```go
import "github.com/go-chi/chi/v5"

r := chi.NewRouter()

// ヘルスチェック
r.Get("/health", healthHandler)

// 認証エンドポイント
r.Route("/auth", func(r chi.Router) {
    r.Get("/login", auth.LoginHandler)
    r.Get("/callback", auth.CallbackHandler)
    r.Get("/me", auth.MeHandler)
    r.Post("/logout", auth.LogoutHandler)
})

// プロキシ（全ての未マッチルートをキャッチ）
r.Handle("/*", proxy.NewHandler(cfg.ProxyTarget))

http.ListenAndServe(cfg.BFFListenAddr, r)
```

### 3. ミドルウェアの活用（将来拡張）

Chi では以下のようなミドルウェアを簡単に追加できます：

```go
import (
    "github.com/go-chi/chi/v5/middleware"
)

r.Use(middleware.RequestID)   // Request ID の自動付与
r.Use(middleware.RealIP)      // Real IP の取得
r.Use(middleware.Logger)      // リクエストログ
r.Use(middleware.Recoverer)   // パニックリカバリ
```

**現時点では追加しない** — 必要になったタイミングで段階的に導入する方針。

### 4. HTTP メソッドの明示化

**Before:**
```go
http.HandleFunc("/auth/logout", auth.LogoutHandler)  // GET/POST 両方受け付ける
```

**After:**
```go
r.Post("/auth/logout", auth.LogoutHandler)  // POST のみ
```

**メリット:**
- RESTful な設計が明確になる
- 意図しないメソッドでのアクセスを防止

### 5. ワイルドカードルーティング

プロキシハンドラーは全ての未マッチルートを処理する必要があるため、`/*` ワイルドカードを使用：

```go
r.Handle("/*", proxy.NewHandler(cfg.ProxyTarget))
```

**注意点:**
- `/*` は最後に定義する（他のルートより優先度が低い）
- `/auth/*` や `/health` などは先に定義されるため、正しく動作する

## Implementation Notes

### 実装ステップ

1. **依存関係追加**
   ```bash
   cd bff
   go get -u github.com/go-chi/chi/v5
   ```

2. **main.go の変更**
   - `chi.NewRouter()` でルーター作成
   - 既存のハンドラー関数はそのまま使用（互換性あり）
   - ルート定義を Chi の形式に変更

3. **テスト実行**
   ```bash
   task test
   ```

4. **動作確認**
   - `/health` エンドポイント
   - `/auth/*` エンドポイント
   - プロキシ機能（`/api/*`）

### 互換性

Chi は `http.HandlerFunc` と `http.Handler` をそのまま受け入れるため、既存のハンドラーコード（`auth/handler.go`, `proxy/handler.go` など）は**変更不要**。

### テストへの影響

既存のテスト（`proxy/handler_test.go` など）は変更不要：
- テストは個別のハンドラー関数を直接テストしている
- ルーティング層の変更は影響しない

## Consequences

**ポジティブ:**
- ✅ ルート定義が明確で読みやすくなる（サブルーター活用）
- ✅ HTTP メソッドの明示化でRESTful設計が強化される
- ✅ 将来的なミドルウェア追加が容易（ロギング、認証、レート制限など）
- ✅ パスパラメータ対応の準備（例: `/users/{userID}`）
- ✅ 標準ライブラリとの互換性により、移行リスクが低い

**ネガティブ:**
- ⚠️ 新しい依存関係の追加
  - **影響**: Chi は軽量で安定しており、Go コミュニティで広く使われている
- ⚠️ 学習コスト
  - **影響**: Chi の API はシンプルで、標準ライブラリに近い設計のため最小限

**リスク評価:**
- 🟢 **低リスク** — Chi は標準ライブラリと互換性があり、既存ハンドラーの変更不要
- 🟢 **高メリット** — コードの可読性と将来の拡張性が向上

**代替案との比較:**

| ルーター | メリット | デメリット |
|---------|---------|----------|
| **標準ライブラリ** (現状) | 依存なし、シンプル | ミドルウェア管理が煩雑、拡張性低い |
| **Chi** (採用) | 軽量、互換性、ミドルウェア充実 | 新しい依存 |
| gorilla/mux | 多機能 | 重い、Chi より遅い |
| Echo/Gin | 高機能フレームワーク | 標準ライブラリとの互換性低い、オーバースペック |

**決定事項:**
- Chi v5 を採用
- 既存ハンドラーはそのまま使用（変更不要）
- ミドルウェアは現時点では追加せず、必要に応じて段階的に導入
