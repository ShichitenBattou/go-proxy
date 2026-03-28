# API

FastAPI + SQLAlchemy の API バックエンド。詳細なアーキテクチャは `CLAUDE.md` を参照。

## ローカル開発環境のセットアップ

### 前提条件

- Docker
- `jq`（トークン取得コマンドで使用、任意）

### 起動

```bash
cd api
docker compose -f compose.dev.yml up --build
```

以下の 3 サービスが立ち上がります。

| サービス | URL | 用途 |
|---|---|---|
| API (FastAPI) | http://localhost:8081 | アプリ本体（ホットリロード有効） |
| PostgreSQL | localhost:5432 | DB（データ永続化あり） |
| mock-oauth2-server | http://localhost:9090 | 認証モック |

ソースコードの変更は自動的に反映されます（`--reload`）。

### マイグレーションの実行

コンテナが起動したら、別ターミナルで実行します。

```bash
docker compose -f compose.dev.yml exec api uv run alembic upgrade head
```

---

## 認証トークンの取得

API の認証には Bearer JWT が必要です。開発環境では **mock-oauth2-server** からトークンを取得します。

```bash
TOKEN=$(curl -s -X POST http://localhost:9090/default/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials&client_id=bff&client_secret=dummy" \
  | jq -r '.access_token')
```

取得したトークンの `sub` クレームを確認します（後続の手順で使用します）。

```bash
echo $TOKEN | cut -d. -f2 | base64 -d 2>/dev/null | jq .sub
```

> **仕組み**: mock-oauth2-server は `client_id` を `aud` クレームに、`SERVER_HOSTNAME` で固定したホスト名を `iss` クレームに設定した JWT を発行します。FastAPI は `KEYCLOAK_ISSUER` / `KEYCLOAK_AUDIENCE` と照合して検証します。

---

## API の使い方

### ユーザーの作成

認証が必要なエンドポイントを使うには、まず対応するユーザーを DB に登録する必要があります。
トークンの `sub` クレームの値を `keycloak_sub` に指定してください。

```bash
SUB=$(echo $TOKEN | cut -d. -f2 | base64 -d 2>/dev/null | jq -r .sub)

curl -s -X POST http://localhost:8081/users/ \
  -H "Content-Type: application/json" \
  -d "{\"keycloak_sub\": \"$SUB\", \"role\": \"USER\"}" | jq .
```

### 投稿の作成

```bash
curl -s -X POST http://localhost:8081/posts/ \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title": "テスト投稿", "body": "本文", "tags": ["dev"]}' | jq .
```

### 投稿一覧の取得

```bash
curl -s http://localhost:8081/posts/ | jq .
```

---

## 停止・データリセット

```bash
# 停止（データは保持）
docker compose -f compose.dev.yml down

# 停止 + DB データも削除
docker compose -f compose.dev.yml down -v
```
