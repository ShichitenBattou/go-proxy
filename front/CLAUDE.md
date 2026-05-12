# front/CLAUDE.md

このディレクトリは静的 HTML/JS/CSS のフロントエンドと NGINX リバースプロキシ設定を管理します。

## ディレクトリ構成

```
front/
├── pages/              # 静的 HTML/JS/CSS ファイル
│   ├── index.html
│   ├── index.css
│   ├── index.js
│   └── signup/
├── default.conf.template  # NGINX 設定テンプレート
├── Taskfile.yml        # TLS 証明書生成タスク
├── san.conf            # Subject Alternative Name 設定
└── rootCA.conf         # Root CA 設定
```

**注**: 証明書ファイル（`*.key`, `*.crt`, `*.csr`）は `.gitignore` で除外されており、Git 管理外です。

## NGINX 設定

`default.conf.template` は以下のルーティングを定義しています:

### `/` — 静的コンテンツ
- `pages/` ディレクトリ内の静的 HTML/JS/CSS を配信
- `index.html` をデフォルトページとして表示
- キャッシュ無効化 (開発時の利便性向上)

### `/idp/` — Keycloak プロキシ
- Keycloak (`keycloak:8080`) へのリバースプロキシ
- OIDC 認証フローで使用

### `/api/` — BFF プロキシ
- bff (`bff:8080`) へのリバースプロキシ
- 認証が必要な API リクエストを転送

## TLS 証明書

HTTPS 通信のために TLS 証明書が必要です。以下の方法で生成できます:

### 方法1: 自己署名証明書 (開発用)

```bash
task generate-keys
```

- `san.conf` に基づいて自己署名証明書を生成
- 生成されるファイル: `key.pem`, `cert.pem`

### 方法2: Root CA + クライアント証明書 (推奨)

```bash
# 1. Root CA の生成
task generate-root-ca

# 2. Root CA で署名されたサーバー証明書の生成
task generate-client-cert
```

- Root CA 証明書をブラウザにインストールすることで、証明書の警告を回避可能
- 生成されるファイル:
  - Root CA: `rootCA.crt`, `rootCA.key`
  - サーバー: `server.crt`, `server.key`, `server.csr`

### 証明書設定ファイル

- **san.conf**: Subject Alternative Name (SAN) 拡張の設定
  - ローカル開発時のホスト名 (例: `localhost`, `auth.local`) を定義
- **rootCA.conf**: Root CA 証明書の設定
  - 組織情報、Common Name などを定義

## 開発コマンド

### 静的ファイルの編集

`pages/` ディレクトリ内のファイルを直接編集してください。変更は NGINX により即座に反映されます (Docker Compose の場合は volume mount が必要)。

### NGINX 設定の変更

1. `default.conf.template` を編集
2. NGINX コンテナを再起動

```bash
docker compose restart front
```

### TLS 証明書の再生成

証明書の有効期限切れや設定変更時:

```bash
cd front
task generate-root-ca        # Root CA の再生成
task generate-client-cert    # サーバー証明書の再生成
```

## Docker Compose での利用

`default.conf.template` は環境変数で設定を動的に変更できます:

- `NGINX_HOST`: サーバー名 (デフォルト: `localhost`)
- `NGINX_PORT`: リッスンポート (デフォルト: `443`)

これらは `.env` ファイルで設定されます。

## 注意事項

- **証明書ファイルの管理**: `server.key`, `rootCA.key` などの秘密鍵は Git にコミットしないこと (`.gitignore` で除外)
- **キャッシュ制御**: 開発時は `Cache-Control: no-cache` が設定されているため、常に最新のファイルが配信される
- **ポート設定**: NGINX は HTTPS (443) でリッスンする。HTTP (80) からのリダイレクト設定は現在未実装
