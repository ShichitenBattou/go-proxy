# ADR-0017: JIT ユーザープロビジョニングと JWT からの keycloak_sub 取得

## Status
承認済み（Accepted） - 2026-05-07

## Context

### 問題 1: フロントエンドが明示的にユーザー登録を呼び出す必要がある

現在、Keycloak でユーザーが登録・ログインを完了しても、API 側の `users` テーブルにはレコードが存在しない。API のユーザーレコードを作成するには、フロントエンドが明示的に `POST /api/users` を呼び出す必要がある。

これにより以下の問題が生じる:

- フロントエンドが「ログイン後に POST /api/users を呼ぶ」という暗黙の契約を持つ
- 呼び出しが漏れた場合、ユーザーが認証済みにもかかわらず API 側に存在しない状態になる
- Keycloak（IDP）と API（リソースサーバー）の結合概念がフロントエンドに露出する

### 問題 2: `keycloak_sub` をリクエストボディで受け取るセキュリティリスク

現在の `POST /api/users` エンドポイントは、リクエストボディで `keycloak_sub` を受け取る:

```python
class CreateUserRequest(BaseModel):
    keycloak_sub: str

@router.post("", status_code=201, response_model=UserResponse)
async def create_user(
    body: CreateUserRequest,   # 任意の値を受け取れる
    current_user: CurrentUser, # JWT 検証済みだが body との突合なし
    ...
```

JWT の `sub` クレームは署名検証済みであるにもかかわらず、ボディの `keycloak_sub` は別途検証されていない。これにより、認証済みユーザーが他人の `keycloak_sub` を指定して別ユーザーのレコードを作成できるリスクがある。

### 選択肢

#### 問題 1 の選択肢

**案A: フロントエンドから明示的に呼び出す（現状維持）**
- フロントエンドがログイン後に `POST /api/users` を呼び出す
- 呼び出し漏れのリスクがある
- フロントエンドに IDP と API の結合概念が漏出する

**案B: BFF の `/auth/callback` 内で API を呼び出す（JIT プロビジョニング）**
- 認証完了のタイミングで BFF が `POST /api/users` を内部呼び出しする
- フロントエンドはユーザー登録を意識しなくてよい
- BFF が API を直接呼び出すため、BFF と API の間に依存が生じる

**案C: API の各エンドポイントでユーザーが存在しない場合に upsert する**
- 全エンドポイントに横断的な処理が必要
- ドメインロジックが散在し、クリーンアーキテクチャの方針に反する

#### 問題 2 の選択肢

**案A: リクエストボディから `keycloak_sub` を取得する（現状維持）**
- 任意の値を渡せるためセキュリティリスクがある
- フロントエンドが `keycloak_sub` を知っている必要がある

**案B: JWT（`current_user.sub`）から `keycloak_sub` を取得する**
- JWT は Keycloak の署名により保護されており、改ざん不可
- リクエストボディが不要になりインターフェースがシンプルになる
- セキュリティの信頼できる唯一の情報源が JWT に統一される

## Decision

**問題 1: 案B（JIT プロビジョニング）** を採用する。
**問題 2: 案B（JWT から keycloak_sub 取得）** を採用する。

### 採用理由

- **案B（JIT）**: BFF はすでに API に HTTP リクエストをプロキシする役割を持っており、callback 内での内部呼び出しはアーキテクチャ上の新たな責務追加ではなく、初回ログイン時の初期化処理として自然に位置づけられる。フロントエンドと IDP の分離が明確になる。
- **案B（JWT）**: 署名検証済みの JWT から取得することで、外部からの不正な値の注入を排除できる。API のインターフェースが簡潔になる。

### 変更後のフロー

```
ブラウザ → NGINX → Keycloak（登録・ログイン）
  → /api/auth/callback（BFF）
      1. State 検証（CSRF 対策）
      2. 認可コード交換（bff トークン取得）
      3. Token Exchange（api トークン取得）
      4. ID Token 検証・claims 抽出
      5. セッション作成・Cookie 設定
      6. [NEW] POST /users を内部呼び出し（JIT プロビジョニング）
           → API が JWT の sub からユーザーを upsert
  → 元の URL へリダイレクト
```

### 変更概要

#### BFF: `auth/callback_handler.go`

セッション作成後、`POST /api/users` を内部 HTTP リクエストで呼び出す関数 `provisionUser` を追加する。

```go
// auth/provision.go（新規）
func provisionUser(ctx context.Context, apiAccessToken string) error {
    // PROXY_TARGET に対して POST /users を送信
    // Authorization: Bearer <apiAccessToken>
    // リクエストボディは不要（API 側が JWT の sub から取得するため）
    // エラー時はログして処理を継続（ソフトフェイル）
}
```

`callback_handler.go` のセッション保存後に呼び出す:

```go
// セッション保存後
if err := provisionUser(ctx, apiAccessToken); err != nil {
    // エラーはログのみ。認証フロー自体は失敗させない
    slog.Error("Failed to provision user", "sub", claims.Sub, "error", err)
}

// Cookie 設定・リダイレクト処理（変更なし）
```

**ソフトフェイルを採用する理由**: API の一時的な障害でログイン自体が不可能になることを防ぐ。API が回復した時点での最初のリクエスト（または再ログイン）で再度プロビジョニングが試みられる（`CreateUserInteractor` がべき等のため）。

#### API: `app/presentation/routers/users.py`

リクエストボディ（`CreateUserRequest`）を削除し、JWT の `sub` クレームを使用する:

```python
@router.post("", status_code=201, response_model=UserResponse)
async def create_user(
    current_user: CurrentUser,  # JWT 検証済み（sub クレームを含む）
    interactor: Annotated[CreateUserInteractor, Depends(get_create_user_interactor)],
) -> UserResponse:
    user = await interactor.execute(CreateUserCommand(keycloak_sub=current_user.sub))
    return UserResponse(...)
```

`CreateUserRequest` クラスは削除する。

## Consequences

**ポジティブ:**
- フロントエンドがユーザー登録を意識する必要がなくなる
- IDP（Keycloak）と API の結合概念がフロントエンドに漏れない
- `keycloak_sub` の不正注入リスクが排除される
- API の `POST /users` インターフェースがシンプルになる
- `CreateUserInteractor` がべき等のため、複数回呼び出されても安全

**ネガティブ:**
- BFF の callback 処理に API への HTTP リクエストが追加され、レイテンシがわずかに増加する
- API が停止している場合、ユーザーはログインできるが API 側にレコードが存在しない状態になる（ソフトフェイルのトレードオフ）
- BFF が API のエンドポイント（`/users`）に依存するため、エンドポイント変更時に BFF も更新が必要

## Implementation Notes

- `provisionUser` の呼び出し先は既存の `PROXY_TARGET` 環境変数（`api:8081`）を使用する
- BFF 内の内部 HTTP クライアントは `utils.GetInternalHTTPClient()` を使用する（既存の TLS 設定を引き継ぐ）
- `POST /users` は認証必須エンドポイントのため、`Authorization: Bearer <apiAccessToken>` ヘッダーを付与する
- API 側の `CurrentUser` 型に `sub` フィールドが存在することを確認してから実装すること（`app/presentation/dependencies.py` 参照）
- `CreateUserRequest` 削除に伴い、既存のテストコードも合わせて修正する
