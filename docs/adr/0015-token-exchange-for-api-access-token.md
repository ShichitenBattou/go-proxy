# ADR-0015: Keycloak Token Exchange による API アクセストークン取得

## Status
承認済み（Accepted） - 2026-05-01

## Context

### 問題

BFF は `bff` Keycloak クライアントで Authorization Code フローを実行し、`oauth2Config.Exchange` でアクセストークンを取得している（`auth/oauth2.go`、`auth/callback_handler.go:67`）。

このトークンは `bff` クライアント向けに発行されるため、`aud` クレームが `bff` になる。一方、API サーバー（FastAPI）は `api` Keycloak クライアントをリソースサーバーとして使用しており、受け取ったトークンのオーディエンスを検証する。

```
oauth2Config.Exchange(ctx, code)
  → Keycloak 発行トークン: aud=["bff"]

proxy/handler.go:38-40
  → Authorization: Bearer <bff トークン>
  → API が aud=["api"] を要求する場合、拒否される
```

現状、セッションには `bff` 向けトークンが `AccessToken` として保存され（`redis.SessionData`）、プロキシがそれをそのまま転送している。

### 選択肢

**案A: Audience Mapper（Keycloak 設定のみ）**
`bff` クライアントに Audience Mapper を追加して、発行トークンの `aud` に `api` を含める。

- コード変更不要だが、`bff` と `api` トークンの分離が失われる

**案B: Keycloak Token Exchange（本 ADR の対象）**
`bff` クライアントのトークンを Keycloak の Token Exchange エンドポイントで `api` 向けトークンに交換する。

- `bff` / `api` のトークンを明確に分離でき、最小権限の原則に沿う

## Decision

**案B: Keycloak Token Exchange** を採用する。

### フロー（変更後）

```
1. ユーザーが /auth/login にアクセス
2. BFF が bff クライアントで認可コードフローを開始（変更なし）
3. Keycloak がコールバックに認可コードを返す（変更なし）
4. BFF が oauth2Config.Exchange でコードを交換 → bff アクセストークン取得（変更なし）
5. [NEW] BFF が Token Exchange エンドポイントを呼び出す
     POST /realms/go-proxy/protocol/openid-connect/token
     grant_type=urn:ietf:params:oauth2:grant-type:token-exchange
     subject_token=<bff アクセストークン>
     subject_token_type=urn:ietf:params:oauth2:token-type:access_token
     requested_token_type=urn:ietf:params:oauth2:token-type:access_token
     audience=api
     client_id=bff
     client_secret=<bff シークレット>
6. Keycloak が api 向けアクセストークンを返す
7. セッションに api アクセストークンを保存（AccessToken フィールド）
8. プロキシが api アクセストークンを Bearer として転送（変更なし）
```

### 変更範囲

#### Keycloak 設定（`keycloak/data/import/realm.json`）

1. **`bff` クライアントにシークレットを追加**
   - 現在 `bff` クライアントにシークレットがない。Token Exchange の `client_secret` 認証に必要なため追加する。

2. **`api` クライアントの Fine-Grained Authorization を有効化**
   - `api` クライアントに Token Exchange ポリシーを追加し、`bff` クライアントからの交換を許可する。
   - Keycloak の管理コンソール: `api` クライアント → Permissions → `token-exchange` パーミッションを有効化 → `bff` クライアントを許可するポリシーを設定。

3. **Token Exchange 機能の有効化**
   - Keycloak v21 以降、Token Exchange はプレビュー機能のため、`keycloak.conf` または起動オプションで有効化が必要な場合がある。
   - `--features=token-exchange` を Docker Compose の Keycloak コマンドに追加する（または `keycloak.conf` に記述）。

#### BFF コード（`auth/callback_handler.go`）

`oauth2Config.Exchange` 後に Token Exchange を実行する関数 `exchangeForAPIToken` を `auth/oauth2.go` に追加する。

```go
// auth/oauth2.go に追加
func exchangeForAPIToken(ctx context.Context, bffAccessToken string) (string, error) {
    cfg := setup.GetConfig()
    // Token Exchange リクエスト
    // ...
    // api アクセストークンを返す
}
```

`callback_handler.go` の `sessionData.AccessToken` に、`bff` トークンではなく交換後の `api` アクセストークンを設定する。

```go
// callback_handler.go（変更後イメージ）
apiAccessToken, err := exchangeForAPIToken(ctx, token.AccessToken)
if err != nil {
    // エラー処理
}
sessionData := redis.SessionData{
    ...
    AccessToken: apiAccessToken, // api 向けトークン
}
```

### 変更しないもの

- `proxy/handler.go` — Bearer トークンの転送ロジックは変更不要
- `redis.SessionData` — フィールド定義は変更不要（`AccessToken` の意味が変わるだけ）
- ログインフロー（`login_handler.go`）、セッション管理、Cookie 処理

## Consequences

**ポジティブ:**
- `bff` と `api` のトークンが明確に分離される（最小権限の原則）
- `api` クライアントは自身のオーディエンスを持つトークンのみ受け付ける
- プロキシ層（`proxy/handler.go`）の変更が不要

**ネガティブ:**
- Keycloak の Token Exchange を有効化する設定変更が必要
- コールバック処理に追加の HTTP リクエスト（Token Exchange）が発生する
- `bff` クライアントにシークレットを追加することで、クライアントが Confidential になる（Public → Confidential の変更）

## Implementation Notes

- Token Exchange エンドポイント: `{OIDC_PROVIDER_URL}/protocol/openid-connect/token`
- `OIDC_PROVIDER_URL` は既に `setup/config.go` で管理されているため、エンドポイント URL はそこから導出する
- Token Exchange レスポンスにはリフレッシュトークンが含まれない場合がある（設計上、`api` トークンのリフレッシュは別途検討が必要）
- `bff` クライアントのシークレットは `OAUTH2_CLIENT_SECRET` 環境変数で管理する（既存の設定キーを使用）
- Keycloak の Fine-Grained Authorization は `realm-management` クライアントへのアクセスが必要なため、`realm.json` インポート後に管理コンソールで手動設定が必要な場合がある。可能であれば `realm.json` に直接エクスポートして管理する。
