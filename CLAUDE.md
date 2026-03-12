# CLAUDE.md

このファイルは、リポジトリで作業する Claude Code (claude.ai/code) へのガイダンスを提供します。

## アーキテクチャ概要

Keycloak を使った認証付き BFF (Backend for Frontend) プロキシパターンのマルチサービス構成です。

- **bff/** — Go 製リバースプロキシ (TLS, ポート 443)。リクエストを受け取り、Redis でセッション管理し、`/api` プレフィックスを除去して API サービスへ転送する。`/auth/login` で Keycloak OIDC ログインへのリダイレクトも担う。
- **api/** — シンプルな Go HTTP バックエンド (ポート 8081)。BFF の転送先。
- **front/** — Next.js 16 / React 19 / Tailwind CSS 4 フロントエンド (開発時ポート 3000)。
- **redis** — セッションストア。セッションは SHA-256 ハッシュ済みキー (`session:<hash>`) で保存される。
- **keycloak** — OIDC アイデンティティプロバイダー (ポート 8082)、PostgreSQL バックエンド。レルム設定は `keycloak/data/import/realm-export.json` からインポート (レルム: `myrealm`、クライアント: `bff`)。

### リクエストフロー

```
ブラウザ → BFF (:443, HTTPS) → API (:8081, HTTP)
                  ↕
            Redis (セッションストア)
```

BFF の動作:
1. リクエストごとに `Session-Id` Cookie を Redis で検証
2. `/api` プレフィックスを除去して `api:8081` へ転送
3. レスポンスのたびにセッション Cookie をローテーション
4. 未認証ユーザーを Keycloak へリダイレクト

### TLS 証明書

BFF は bff ディレクトリ内の `./_keys/server.crt` と `./_keys/server.key` を必要とする。以下で生成:

```bash
task generate-keys
```

## 開発コマンド

### フルスタック (Docker Compose)

```bash
cp .env.example .env
# .env の HOST_WORKSPACE をホストマシン上のリポジトリの絶対パスに変更する

docker compose up --build
```

### BFF (Go)

```bash
cd bff
go run main.go          # ローカル起動 (redis:6379 が必要)
go build ./...
go test ./...           # 全パッケージテスト
go test ./redis/...     # 特定パッケージのテスト
```

### API (Go)

```bash
cd api
go run main.go
go test ./...
```

### フロントエンド (Next.js)

```bash
cd front
npm install
npm run dev             # ホットリロード付き開発サーバー
npm run build
npm run lint
```

### ツール類

```bash
task semgrep            # Semgrep による静的解析
task generate-keys      # 自己署名 TLS 証明書・鍵ペアを ./keys/ に生成
```

ツールバージョンは [aqua](https://aquaproj.github.io/) (`aqua.yaml`) で管理。devcontainer セットアップ後、`aqua i` で全ツール (uv, task, github-mcp-server) がインストールされる。

## BFF アーキテクチャ方針

BFF は **Vertical Slice Architecture** で改修していく予定。機能・ユースケース単位 (proxy, auth, session など) でコードをまとめ、技術レイヤー単位のグループ化は避ける。新機能追加・修正時は、ハンドラー・ロジック・データアクセスを 1 つのスライス (ディレクトリ or パッケージ) に収める。

## 注意事項

- BFF のモジュール名は `bff` (`bff/go.mod` 参照)。内部パッケージのインポートは `bff/redis` のように行う。
- `bff/redis/redis.go` の Redis クライアントは呼び出しごとに生成される (コネクションプールなし)。
- Keycloak の管理者認証情報のデフォルトは `admin/admin`、DB は `postgres/postgres/postgres`。
- `.env` の `HOST_WORKSPACE` は **ホストマシン** 上のリポジトリ絶対パスを指定する (Keycloak レルムインポート用 Docker ボリュームマウントに使用)。
- `compose.yml` は Postgres 永続化に `pgdata` ボリュームを使用。状態をリセットする場合は削除が必要。
