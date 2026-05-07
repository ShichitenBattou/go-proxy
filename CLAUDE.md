# CLAUDE.md

このファイルは、リポジトリで作業する Claude Code (claude.ai/code) へのガイダンスを提供します。

## 開発ルール（厳守）

- **実装時には ADR (Architecture Decision Record) を作成して認識合わせをすること**
  - いかなる場合でも事前に ADR を作成する
  - ADR は `docs/adr/` ディレクトリに配置する
  - ADR のフォーマットや詳細は既存の ADR ファイルを参考にすること。
  - ADR を作成したら即時コミットし、**必ずユーザーの承認を得てから実装を開始すること**                                       
  - ADR 作成後は実装に進まず、ユーザーのレビューを待つこと 
- 作業開始前にserena MCPをアクティベートすること。
- 作業後は必ず`task ci`を実行し、エラーが無いかを確認する事。

## アーキテクチャ概要

Keycloak を使った認証付き BFF (Backend for Frontend) プロキシパターンのマルチサービス構成です。

- **front/** — 静的 HTML/JS + NGINX リバースプロキシ (HTTPS/TLS)。静的コンテンツの配信と、API リクエストを bff へルーティング。詳細は `front/CLAUDE.md` 参照。
- **bff/** — Go 製認証プロキシ (oauth2-proxy ライクな動作)。NGINX からのリクエストを受け取り、Redis でセッション管理し、`/api` プレフィックスを除去して API サービスへ転送。未認証ユーザーを Keycloak OIDC ログインへリダイレクト。詳細は `bff/CLAUDE.md` 参照。
- **api/** — Python/FastAPI バックエンド。クリーンアーキテクチャ + CQRS を採用。詳細は `api/CLAUDE.md` 参照。
- **redis** — セッションストア。セッションは SHA-256 ハッシュ済みキー (`session:<hash>`) で保存される。
- **keycloak** — OIDC アイデンティティプロバイダー、PostgreSQL バックエンド。レルム設定は `keycloak/data/import/realm-export.json` からインポート (レルム: `myrealm`、クライアント: `bff`)。

### リクエストフロー

```
ブラウザ → NGINX (HTTPS) → bff (Go) → API (HTTP)
               ↓              ↕
           静的HTML       Redis (セッションストア)
```

**NGINX の役割**:
- `/` — 静的 HTML/JS/CSS の配信
- `/idp/` — Keycloak へのプロキシ
- `/api/` — bff へのプロキシ

**bff の動作**:
1. リクエストごとに `Session-Id` Cookie を Redis で検証
2. `/api` プレフィックスを除去して API へ転送
3. レスポンスのたびにセッション Cookie をローテーション
4. 未認証ユーザーを Keycloak OIDC ログインへリダイレクト

## 開発環境のセットアップ

### 必要なツールのインストール

このプロジェクトは [aqua](https://aquaproj.github.io/) でツールバージョンを一元管理しています。
以下のツールが `aqua.yaml` で定義されています:

- **uv** (0.10.7) — Python パッケージ管理
- **task** (v3.48.0) — タスクランナー
- **go** (go1.26.1) — Go 言語
- **lefthook** (v2.1.4) — Git hooks
- **github-mcp-server** (v0.31.0) — MCP サーバー

```bash
# 1. aqua のインストール (初回のみ)
# https://aquaproj.github.io/docs/tutorial/quick-start#install-aqua

# 2. ツールのインストール
aqua install

# または、プロジェクト全体のセットアップ (aqua install + 各サービスの依存関係)
task setup
```

### Taskfile について

よく使うコマンドは各ディレクトリの `Taskfile.yml` に定義されています。

- **実行方法**: `task <タスク名>` (例: `task setup`, `task api:test`)
- **タスク一覧**: `task --list` または `task -l`
- **モノレポ対応**: ルートから `task <サービス名>:<タスク名>` で各サービスのタスクを実行可能

## 開発コマンド

### フルスタック起動

```bash
cp .env.example .env
# .env の HOST_WORKSPACE をホストマシン上のリポジトリの絶対パスに変更

docker compose up --build
```

### 各サービスの開発

各サービスの詳細なコマンドとアーキテクチャは、それぞれの CLAUDE.md を参照してください:

- **フロントエンド (Next.js + NGINX)**: `front/CLAUDE.md`
- **BFF (Go)**: `bff/CLAUDE.md`
- **API (Python/FastAPI)**: `api/CLAUDE.md`

### 共通タスク (ルートディレクトリ)

```bash
task setup          # プロジェクト全体のセットアップ (aqua + 全サービス)
task semgrep        # Semgrep による静的解析

# サービス別タスクの実行例
task bff:test       # bff のテストを実行
task api:test       # api のテストを実行
```

## BFF アーキテクチャ方針

BFF は **Vertical Slice Architecture** で構成されている。機能・ユースケース単位でコードをまとめ、技術レイヤー単位のグループ化は避ける。新機能追加・修正時は、ハンドラー・ロジック・データアクセスを 1 つのスライス (ディレクトリ or パッケージ) に収める。

現在のスライス構成:
- **`bff/proxy/`** — `/api/` ルートのリバースプロキシ・セッション検証・ローテーション
- **`bff/auth/`** — `/auth/login`・`/auth/callback` の認証フロー
- **`bff/redis/`** — セッション永続化インフラ (各スライスから利用される共有層)

## 注意事項

- BFF のモジュール名は `bff` (`bff/go.mod` 参照)。内部パッケージのインポートは `bff/redis` のように行う。
- `bff/redis/redis.go` の Redis クライアントは呼び出しごとに生成される (コネクションプールなし)。接続先は `REDIS_ADDR` 環境変数で上書き可能 (デフォルト: `redis:6379`)。テスト時は `bff/compose.test.yml` で Redis を起動し、`task test` が自動で設定する。
- Keycloak の管理者認証情報のデフォルトは `admin/admin`、DB は `postgres/postgres/postgres`。
- `.env` の `HOST_WORKSPACE` は **ホストマシン** 上のリポジトリ絶対パスを指定する (Keycloak レルムインポート用 Docker ボリュームマウントに使用)。
- `compose.yml` は Postgres 永続化に `pgdata` ボリュームを使用。状態をリセットする場合は削除が必要。

### 既知の技術的負債

以下は把握済みの課題。新機能開発より先に対処すること（特に中優先度以上）。

#### #5 レスポンスボディ未読によるコネクション再利用不可 — `bff/auth/provision.go`（中）

現在はエラー時にボディを読んでクローズしているが、成功時も `io.ReadAll` で消費している。
HTTP/1.1 ではボディ全体を消費してからクローズしないとコネクションが再利用されない点に注意。
将来的にコネクションプールを導入する場合は、全パスで確実にボディを消費していることを確認すること。

#### #6 API テストに旧仕様の痕跡 — `api/tests/presentation/test_users.py`（中）

`TestGetUser` 内で `POST /users` を呼ぶ際に `json={"keycloak_sub": "..."}` を渡しているケースが残存している。
現在の実装はリクエストボディを無視するため動作はするが、テストが実装と乖離している。
`test_users.py` を修正する際は、`POST /users` の呼び出しからボディを削除すること。

#### #7 冪等 POST のステータスコード — API / BFF（中）

`POST /users` は既存ユーザーの場合も `201 Created` を返す（HTTP セマンティクス上は `200 OK` が適切）。
`provision.go` 側は `201` のみを成功とみなすよう実装済み。
将来 API の挙動を変える場合は BFF 側の期待ステータスコードも合わせて変更すること。

#### #8 Redis にトークンが平文保存 — `bff/redis/redis.go`（中）

`SessionData` の `AccessToken` / `RefreshToken` が平文で Redis に保存されている。
Redis が侵害された場合、全ユーザーのトークンが漏洩しユーザーのなりすましが可能になる。
本番投入前に Redis の TLS 有効化・ACL 設定・ネットワーク分離を必ず実施すること。

#### #9 Redis クライアントがリクエストごとに生成 — `bff/redis/redis.go`（低）

`createClient()` が全 Redis 操作で毎回呼ばれコネクションプールがない。
高トラフィック時にコネクション数が増加し、パフォーマンスが劣化する。
`sync.Once` でシングルトンクライアントを生成する方式に移行することが望ましい。

#### #10 エラー詳細が HTTP レスポンスに露出 — `api/app/presentation/auth.py`（低）

`jwt.InvalidTokenError` の詳細が `detail=f"Invalid token: {str(e)}"` でそのままクライアントに返される。
攻撃者に JWT 検証ライブラリの内部情報やトークン形式のヒントを与えうる。
本番環境では `"Invalid token"` のような固定メッセージに差し替えること（`settings.stage` で分岐可能）。
