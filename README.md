# go-proxy

Keycloak OIDC 認証付き BFF (Backend for Frontend) リバースプロキシの POC 実装。

「せっかくだからちゃんとやる」をコンセプトに、ルート CA 証明書の生成から Token Exchange まで、実際のプロダクション構成に近い形で組み上げたものです。

## 構成

```
ブラウザ → NGINX (HTTPS/443) → BFF (Go/:8080) → API (FastAPI/:8081)
               ↓                    ↕
           静的 HTML/JS         Redis (セッションストア)
               ↓
           Keycloak (/idp/)
```

| サービス | 技術 | 役割 |
|---|---|---|
| **front** | NGINX | HTTPS 終端・静的配信・リバースプロキシ |
| **bff** | Go | 認証プロキシ・セッション管理・リバースプロキシ |
| **api** | Python / FastAPI | バックエンド API |
| **redis** | Redis | セッションストア |
| **keycloak** | Keycloak | OIDC アイデンティティプロバイダー |

## やったこと（面倒だったところ）

### TLS / ルート CA 込みの HTTPS 構成

`front/` に自前のルート CA を生成するタスクを用意し、そのルート CA で署名したサーバー証明書を NGINX に設定しています。ブラウザにルート CA をインストールすれば証明書警告なしで動作します。

BtoC サービスであれば公的 CA で済む話ですが、社内システム的には自前 CA を使うケースがあるため、ブラウザだけでなく各サービスのHTTPクライアントにも同じルート CA を信頼させています。

- **BFF (Go)**: `ROOT_CA_FILE` 環境変数でルート CA を読み込み、Keycloak への内部 HTTPS 通信で使用するカスタム HTTP クライアントに設定
- **API (Python)**: `JWKS_SSL_CA_BUNDLE` 環境変数で `PyJWKClient` の `ssl_context` に渡し、Keycloak の JWKS エンドポイントへの接続で使用（ADR-0013）

```bash
cd front
task generate-root-ca      # rootCA.crt / rootCA.key を生成
task generate-client-cert  # server.crt / server.key を生成
```

### Keycloak Token Exchange（ADR-0015）

BFF は `bff` クライアントで認可コードフローを実行し、取得したアクセストークンを Keycloak の Token Exchange エンドポイントで `api` クライアント向けに交換します。これにより `bff` と `api` のトークンが明確に分離され、最小権限の原則に沿った構成になっています。

```
認可コードフロー → bff トークン (aud=bff)
                       ↓ Token Exchange
                 api トークン (aud=api) → API へ転送
```

### Redis セッション暗号化（AES-256-GCM）（ADR-0022）

Redis が侵害されても OAuth2 トークンが即座に漏洩しないよう、セッションデータを AES-256-GCM で暗号化して保存しています。認証付き暗号（AEAD）なので改ざん検知も兼ねます。セッションキーは SHA-256 ハッシュ済みで保存されます。

```bash
# キー生成
openssl rand -hex 32
```

### リアクティブなトークンリフレッシュ（ADR-0020）

API から 401 が返ってきたときのみ、セッションに保存した Refresh Token を使って Access Token を透過的に更新し、同一リクエストをリトライします。不要な Redis 書き込みを避けつつ、ユーザーに再ログインを強いない設計です。

### ログインフローのセキュリティ強化（ADR-0008）

- **State トークン TTL**: 5 分で自動失効（Redis TTL + アプリ層の二重チェック）
- **オープンリダイレクト対策**: プロトコル相対 URL (`//evil.com`) や改行文字をブロック、正規表現ホワイトリストで許可パスを制限

### Keycloak 起動待ち画面（ADR-0016）

Keycloak の起動には時間がかかるため、`/idp/` への接続失敗時（502/503/504）に NGINX が専用の待機ページを返すよう設定しています。素の Bad Gateway をそのまま出さない、地味だけど体験に効く対応です。

### JIT ユーザープロビジョニング（ADR-0017）

初回ログイン時、Keycloak の `sub` クレームをキーに API 側でユーザーレコードを自動作成します。プロビジョニング状態はセッションにフラグとして持ち、失敗時は次回リクエスト時に再試行します（ADR-0018）。

## セットアップ

### 必要なもの

- Docker / Docker Compose
- [aqua](https://aquaproj.github.io/) (ツールバージョン管理)

```bash
# ツールのインストール
aqua install

# または全依存関係を一括セットアップ
task setup
```

### TLS 証明書の生成

```bash
cd front
task generate-root-ca      # rootCA.crt / rootCA.key を生成
task generate-client-cert  # server.crt / server.key を生成
```

生成した `rootCA.crt` をブラウザ / OS にインストールしてください。

### 起動

各ディレクトリに `.env.example` があるので、それぞれコピーして設定します。

```bash
cp .env.example .env
cp api/.env.example api/.env
cp bff/.env.example bff/.env
# 各 .env を環境に合わせて編集
```

`https://auth.local` にアクセスできるよう `/etc/hosts` に以下を追加してください。

```
127.0.0.1 auth.local
```

#### 初回起動

DB ボリュームのリセット・マイグレーション・Keycloak の起動待機まで一括で行います。

```bash
task reset_and_run
```

Keycloak の起動には時間がかかります。起動完了前にブラウザでアクセスすると、NGINX が専用の「起動待ち」ページを表示します（ADR-0016）。

#### 2 回目以降

```bash
docker compose up
```

### BFF の環境変数（主要なもの）

`bff/.env.example` を `bff/.env` にコピーして設定します。

| 変数 | 説明 |
|---|---|
| `REDIS_ENCRYPTION_KEY` | AES-256-GCM キー（必須）: `openssl rand -hex 32` で生成 |
| `OAUTH2_CLIENT_SECRET` | Keycloak `bff` クライアントのシークレット |
| `SESSION_TTL` | セッション有効期間（デフォルト: `720h`） |
| `ALLOWED_REDIRECT_PATH_PATTERN` | ログイン後リダイレクト先のホワイトリスト（正規表現） |

## 開発

```bash
task bff:test   # BFF のテスト（Redis を自動起動/停止）
task api:test   # API のテスト
task ci         # 全サービスの CI チェック
```

API 単体は `api/compose.dev.yml` で起動でき、mock-oauth2-server を使って認証なしで動作確認できます。詳細は `api/README.md` を参照。

## アーキテクチャ方針

- **BFF**: Vertical Slice Architecture — `auth/`, `proxy/`, `redis/` の各スライスが独立
- **API**: Clean Architecture + CQRS — `domain` / `application` / `infrastructure` / `presentation` の 4 層構造

## ADR

設計上の意思決定は `docs/adr/` に記録しています。

| # | タイトル |
|---|---|
| 0004 | chi ルーターへの移行 |
| 0008 | ログインフローのセキュリティ強化（State TTL・オープンリダイレクト対策） |
| 0009 | セッション戦略の改訂 |
| 0011 | 公開エンドポイント用プロキシハンドラー |
| 0015 | Keycloak Token Exchange による API アクセストークン取得 |
| 0017 | JIT ユーザープロビジョニング |
| 0018 | セッションプロビジョニングフラグによる遅延再プロビジョニング |
| 0020 | リアクティブなトークンリフレッシュ（API 401 契機） |
| 0022 | Redis セッションデータの AES-256-GCM 暗号化 |
