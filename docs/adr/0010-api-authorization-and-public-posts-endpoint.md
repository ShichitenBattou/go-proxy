# ADR-0010: API エンドポイントの認可必須化と公開投稿一覧エンドポイントの追加

## Status
承認済み（Accepted） - 2026-04-26

## Context

現在、API サービス（FastAPI）の多くのエンドポイントは認可なしでアクセス可能な状態になっている。

### 現在の認可状況

| エンドポイント | 認可 |
|---|---|
| `POST /posts` | ✅ 必要（`CurrentUser`） |
| `GET /posts` | ❌ 不要 |
| `GET /posts/{post_id}` | ❌ 不要 |
| `POST /users` | ❌ 不要 |
| `GET /users/{user_id}` | ❌ 不要 |
| `DELETE /users/{user_id}` | ❌ 不要 |

### 要件

1. 全エンドポイントを認可必須にする
2. 投稿一覧を認可なしで取得できる公開エンドポイント `GET /public/posts/` を別途追加する

`CurrentUser` は `Authorization: Bearer <JWT>` ヘッダーを検証し、DB に登録済みのユーザーを要求する（`dependencies.py:get_current_user`）。

## Decision

### 1. 全エンドポイントを認可必須化

すべての既存エンドポイントのハンドラー引数に `current_user: CurrentUser` を追加する。

- `users.py`: `create_user`, `get_user`, `deactivate_user`
- `posts.py`: `list_posts`, `get_post`（`create_post` は既に対応済み）

`current_user` を実際の処理に使わないエンドポイントでも、依存関係として宣言することで認可チェックを強制する。

### 2. 公開エンドポイント `GET /public/posts/` の追加

- 新規ルーターファイル `api/app/presentation/routers/public.py` を作成する
- プレフィックスは `/public`、エンドポイントは `GET /posts`
- `ListPostsInteractor` を使い、`GET /posts` と同一のクエリロジックを実行する
- 認可チェック（`CurrentUser`）は不要

`main.py` に `public.router` を追加登録する。

### ディレクトリ構成

```
routers/
├── posts.py           # 認可必須の投稿エンドポイント
├── users.py           # 認可必須のユーザーエンドポイント
└── public/
    ├── __init__.py    # 公開ルーターを集約（main.py の変更なしに拡張可能）
    └── posts.py       # 認可不要の公開投稿エンドポイント
```

`public/__init__.py` でサブルーターを集約することで、`public/users.py` 等が将来追加されても `main.py` を変更せずに済む。

### 採用しなかった代替案

**A. posts.py に直接追加する案**
`/public/posts` は `posts.py`（プレフィックス `/posts`）に収まらないため、別ルーターが適切。

**B. posts.py に別プレフィックスのルーターを追加する案**
1 ファイルに複数のプレフィックスを持たせるとルーターの責務が不明確になる。

**C. `public.py` 単一ファイルの案**
将来の公開エンドポイント追加時に肥大化する。`public/` ディレクトリで分割する方が可読性・保守性に優れる。

## Consequences

**ポジティブ:**
- ✅ 全エンドポイントが一貫して認可済みユーザーのみ利用可能になる
- ✅ 公開投稿一覧は認証なしでも取得可能（SEO・外部連携向け）
- ✅ 公開エンドポイントと保護エンドポイントがファイル単位で明確に分離される

**ネガティブ:**
- ⚠️ `create_user`（`POST /users`）を認可必須にすると、未登録ユーザーが自己登録できなくなる
  - BFF がユーザー登録を代理する設計（Keycloak 認証後に BFF が `POST /users` を呼ぶ想定）であれば問題ない
  - 運用フローの確認が必要

## Implementation Notes

- `api/app/presentation/routers/users.py`: 各ハンドラーに `current_user: CurrentUser` を追加
- `api/app/presentation/routers/posts.py`: `list_posts`, `get_post` に `current_user: CurrentUser` を追加
- `api/app/presentation/routers/public/__init__.py`: 新規作成、公開ルーターを集約
- `api/app/presentation/routers/public/posts.py`: 新規作成、`GET /posts` を公開エンドポイントとして実装
- `api/main.py`: `public.router` を `app.include_router` に追加
