# ADR-0021: 非アクティブユーザーのリソース操作制限

## Status

提案中（Proposed） - 2026-05-11

## Context

### 現状

`is_active=False` のユーザーに対するアクセス制限は、現在 `CreatePostInteractor` のみで実施されている。

| エンドポイント | 現状の `is_active` チェック |
|---|---|
| `POST /posts` | ✅ `CreatePostInteractor` で 403 |
| `GET /posts` | ❌ チェックなし |
| `GET /posts/{post_id}` | ❌ チェックなし |
| `GET /users/{user_id}` | ❌ チェックなし |
| `DELETE /users/{user_id}` | ❌ チェックなし |
| `POST /users` | 対象外（`CurrentSub` のみ使用・JIT プロビジョニング用） |

`is_active=False` のユーザーは無効化されたアカウントであるため、**すべての認証付きリソース操作をブロックする**べきである。
現状では一部の操作（投稿参照・ユーザー参照・ユーザー無効化）が非アクティブユーザーでも実行可能な状態になっている。

## Decision

**`get_current_user` 関数に `is_active` チェックを追加し、非アクティブユーザーをすべての `CurrentUser` 依存エンドポイントからブロックする。**

### 変更概要

#### `app/presentation/dependencies.py`

`get_current_user` の取得後に `is_active` を検証する:

```python
async def get_current_user(
    keycloak_sub: CurrentSub,
    user_repo: UserRepoDep,
) -> User:
    user = await user_repo.get_by_keycloak_sub(keycloak_sub)
    if not user:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="User not registered. Please register at POST /users/ first.",
            headers={"WWW-Authenticate": "Bearer"},
        )
    if not user.is_active:
        raise HTTPException(
            status_code=status.HTTP_403_FORBIDDEN,
            detail="Inactive users cannot perform resource operations.",
        )
    return user
```

### 採用しなかった代替案

**A. 各インタラクターで個別チェックする案**

- `CreatePostInteractor` では既に実施されており、このパターンを他のインタラクター（クエリ含む）に拡散すると、チェック漏れが生じやすい。
- クエリ（`GetPostInteractor`, `ListPostsInteractor`, `GetUserInteractor`）はユーザー情報を受け取らない設計のため、追加のリポジトリ呼び出しが必要になりコストが高い。

**B. ミドルウェアで一律チェックする案**

- `POST /users` は `CurrentSub` を使う（`CurrentUser` を持たない）ため、ミドルウェアでは区別が難しい。
- `get_current_user` での検証の方が FastAPI の DI に自然に適合する。

### `CreatePostInteractor` の既存チェックについて

`CreatePostInteractor` 内の `is_active` チェックは冗長になるが、ドメイン層の防御的プログラミングとして残す。プレゼンテーション層とドメイン層で二重に保護する。

## Consequences

**ポジティブ:**
- 非アクティブユーザーの制限が一か所（`get_current_user`）に集約され、漏れが生じにくい。
- `POST /users` は対象外のため、JIT プロビジョニングフローに影響しない。

**ネガティブ:**
- `CreatePostInteractor` の `is_active` チェックが冗長になる（ただし意図的に残す）。

## Implementation Notes

- `api/app/presentation/dependencies.py`: `get_current_user` に `is_active` チェックを追加
- `api/tests/presentation/test_users.py` / `test_posts.py`: 非アクティブユーザーが各エンドポイントで 403 を返すことを確認するテストを追加
