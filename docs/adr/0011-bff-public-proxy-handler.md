# ADR-0011: BFF への公開エンドポイント用プロキシハンドラーの追加

## Status
承認済み（Accepted） - 2026-04-26

## Context

ADR-0010 で API に認可不要の公開エンドポイント `GET /public/posts` が追加された。

現在の BFF のリクエストフローは以下の通り:

```
ブラウザ → NGINX (strips /api/) → BFF (/*) → proxy.NewHandler → API
```

`proxy.NewHandler` は全リクエストに対して以下を行う:

1. `Session-Id` Cookie を Redis で検証
2. セッションが存在する場合、`Authorization: Bearer <AccessToken>` ヘッダーを付与
3. `Request-Id` ヘッダーを付与してAPIへ転送

API の `/public/*` エンドポイントは認可不要であるため、BFF がセッション検証や `Authorization` ヘッダー付与を行う必要はない。現在の catch-all `/*` ハンドラーではセッションがなくてもリクエスト自体は通るが、セッション検証のエラーログが出力され、ロジックの意図が不明瞭になる。

## Decision

### 新しいハンドラー `proxy.NewPublicHandler` を追加する

`proxy/public_handler.go` に `NewPublicHandler(forwardHost string) http.Handler` を実装する。

**既存 `NewHandler` との差分:**
- セッション Cookie の検証を行わない
- `Authorization` ヘッダーを付与しない
- パスをそのまま API へ転送する（`/public/posts` → API の `/public/posts`）
- `Request-Id` ヘッダーは付与する（トレーサビリティのため）
- `Access-Control-Allow-Origin` / `Access-Control-Allow-Methods` ヘッダーはレスポンスに付与する（既存ハンドラーと統一）

### main.go のルーティング変更

chi ルーターに `/public/*` ルートを catch-all `/*` より前に登録する。

```go
r.Handle("/public/*", proxy.NewPublicHandler(cfg.ProxyTarget))
r.Handle("/*", proxy.NewHandler(cfg.ProxyTarget))  // 既存（認証あり）
```

### リクエストフロー（変更後）

```
ブラウザ
  GET /api/public/posts
     ↓
  NGINX (strips /api/)
     ↓
  BFF: GET /public/posts
     ↓ (chi: /public/*)
  proxy.NewPublicHandler
     ↓ (セッション検証なし、Authorizationヘッダーなし)
  API: GET /public/posts
```

### 採用しなかった代替案

**A. `NewHandler` に認証スキップオプションを追加する案**
オプション引数によって同一関数内で分岐する設計は、Vertical Slice Architecture の意図に反し、責務が曖昧になる。公開プロキシと認証プロキシは明確に分離すべき。

**B. API ゲートウェイで `/public` をBFFをバイパスして直接ルーティングする案**
NGINX の設定変更が必要になり、BFF の責務範囲を変える大きな変更になる。BFF 内でハンドラーを追加する方が変更範囲が小さく、一貫性がある。

## Consequences

**ポジティブ:**
- ✅ 公開エンドポイントのリクエストでセッション検証エラーログが出力されなくなる
- ✅ 認証ありプロキシと認証なしプロキシが明確に分離され、意図が明瞭になる
- ✅ Vertical Slice Architecture に沿った実装（プロキシスライス内の責務分離）

**ネガティブ:**
- ⚠️ `/public/*` に一致するパスは認証なしでAPIに到達するため、APIの認可実装が正しく機能していることが前提となる

## Implementation Notes

- `proxy/public_handler.go`: `NewPublicHandler` を新規作成
- `proxy/public_handler_test.go`: パス転送・Authorizationヘッダー不付与・Request-Idヘッダー付与のテストを追加
- `main.go`: `/public/*` ルートを `/*` より前に登録
