# ADR-0019: POST /users の冪等性とステータスコード修正

## Status

承認済み（Accepted） - 2026-05-11

## Context

### 問題: 既存ユーザーへの POST /users が常に 201 を返す

`POST /users` は JIT プロビジョニング用のべき等エンドポイントとして設計されている。`CreateUserInteractor` は既存ユーザーが存在する場合はそのまま返却するが、API ルーターは新規・既存を問わず常に `201 Created` を返す。

HTTP セマンティクスとしては:
- `201 Created`: リソースが新たに作成されたことを示す
- `200 OK`: 既存リソースが返されたことを示す

現状では BFF の `provision.go` が `201` のみを成功とみなしているため、将来的にステータスコードを修正した場合に BFF 側が壊れる。ADR-0018 の実装ノートにも「`CreateUserInteractor` がべき等なので問題なし」と記載されており、API が正確なステータスコードを返すことが前提となっている。

### テストの乖離

`test_create_user_idempotent_returns_201` は 1 回目・2 回目ともに `201` を期待しているが、正しい挙動では 2 回目は `200` であるべき。

## Decision

**新規ユーザー作成時は 201、既存ユーザー返却時は 200 を返す**。

### 変更概要

#### API: `app/application/commands/create_user.py`

`execute` の戻り値を `tuple[User, bool]` に変更し、新規作成かどうかをルーター層に伝える:

```python
async def execute(self, command: CreateUserCommand) -> tuple[User, bool]:
    existing = await self._user_repository.get_by_keycloak_sub(command.keycloak_sub)
    if existing:
        return existing, False  # 既存ユーザー

    user = User(id=uuid4(), keycloak_sub=command.keycloak_sub)
    await self._user_repository.add(user)
    return user, True  # 新規作成
```

#### API: `app/presentation/routers/users.py`

`Response` パラメータを追加してステータスコードを動的に設定する:

```python
@router.post("", status_code=201, response_model=UserResponse)
async def create_user(
    response: Response,
    keycloak_sub: CurrentSub,
    interactor: Annotated[CreateUserInteractor, Depends(get_create_user_interactor)],
) -> UserResponse:
    user, created = await interactor.execute(CreateUserCommand(keycloak_sub=keycloak_sub))
    if not created:
        response.status_code = 200
    return UserResponse(...)
```

#### API: `tests/presentation/test_users.py`

べき等テストを更新する:

```python
async def test_create_user_idempotent(self, client: AsyncClient) -> None:
    first = await client.post("/users")
    second = await client.post("/users")

    assert first.status_code == 201
    assert second.status_code == 200          # 既存ユーザー → 200
    assert first.json()["id"] == second.json()["id"]
```

#### BFF: `auth/provision.go`

`201` のみを成功とみなす判定を `200` も受け入れるよう修正する:

```go
if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
    return fmt.Errorf("provision request returned unexpected status %d: %s", resp.StatusCode, string(body))
}
```

## Consequences

**ポジティブ:**
- HTTP セマンティクスに準拠した正確なステータスコードを返す。
- API と BFF の契約が明確になり、将来的な混乱を防ぐ。
- テストが実装の挙動を正確に反映する。

**ネガティブ:**
- `CreateUserInteractor` の戻り値型が変わるため、他の呼び出し箇所（存在する場合）への影響確認が必要。
- ルーター層で `response.status_code` を直接操作する方式は FastAPI のドキュメント生成に影響しないが、`status_code=201` のデフォルト値が OpenAPI スキーマに反映されたままとなる点に注意。
