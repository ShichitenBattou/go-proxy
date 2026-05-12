# ADR-0023: OpenAPI エンドポイント description の整備

## Status

提案中（Proposed） - 2026-05-12

## Context

### 現状と問題

`api/app/presentation/routers/` 配下の FastAPI ルーター関数には `summary` / `description` が未設定のため、`/docs` (Swagger UI) や `/openapi.json` で生成される OpenAPI スキーマには関数名をそのまま変換した文字列（例: `Create User`, `Deactivate User`）しか表示されない。

特に以下が不明瞭：

| エンドポイント | 問題 |
|---|---|
| `DELETE /users/{user_id}` | 物理削除と誤認される。実態は `is_active=False` にセットする**論理削除（論理無効化）** |
| `POST /users` | 冪等性（同一 `keycloak_sub` で 200 / 新規で 201 を返す）が不明 |
| `GET /users/{user_id}` | 取得可能なユーザー範囲（`is_active=False` でも取得できるか）が不明 |
| `POST /posts` | 投稿可能条件（`role=USER` かつ `is_active=True`）が不明 |
| `GET /posts` | クエリパラメータ `author_id` / `limit` / `offset` の説明が不足 |

### 影響範囲

- 外部クライアント実装者・フロントエンド開発者が API の正確な挙動を把握しにくい
- `DELETE` が論理削除であることを知らずに追加の削除処理を実装するリスクがある

## Decision

**FastAPI の `summary` / `description` パラメータを各エンドポイントデコレータに追加する。**

Python の triple-quote docstring を使った FastAPI の description 記法（`@router.delete(..., description="...")` または関数 docstring）は可読性が低いため、デコレータの `summary` と `description` 引数を明示的に使用する。

### 変更方針

- `summary`: 動詞+名詞の短い英語表記（Swagger UI の一覧表示に使用）
- `description`: Markdown 対応。挙動・制約・副作用を日本語または英語で記述

### 各エンドポイントの記述内容

#### `users.py`

| エンドポイント | summary | description の要点 |
|---|---|---|
| `POST /users` | Upsert user by Keycloak sub | 同一 `keycloak_sub` が既存なら 200、新規作成なら 201 を返す（冪等） |
| `GET /users/{user_id}` | Get user by ID | 指定 ID のユーザーを返す。存在しない場合は 404 |
| `DELETE /users/{user_id}` | Deactivate user (soft delete) | **物理削除ではなく論理削除**。`is_active` を `False` にセットする。データは保持される |

#### `posts.py`

| エンドポイント | summary | description の要点 |
|---|---|---|
| `POST /posts` | Create a post | `role=USER` かつ `is_active=True` のユーザーのみ投稿可能。admin は投稿不可 |
| `GET /posts` | List posts | `author_id`・`limit`・`offset` でフィルタ・ページネーション可能 |
| `GET /posts/{post_id}` | Get post by ID | 指定 ID の投稿を返す。存在しない場合は 404 |

### 採用しなかった方式

| 案 | 理由 |
|---|---|
| 関数 docstring で記述 | FastAPI は docstring を description として使うが、Markdown 長文はコードの可読性を下げる |
| 別ファイル（OpenAPI オーバーライド）で管理 | コードと乖離するリスクが高い |

## Consequences

**ポジティブ:**
- Swagger UI (`/docs`) と ReDoc (`/redoc`) で各エンドポイントの挙動が明示される
- `DELETE /users/{user_id}` が論理削除であることが生成 JSON から直接読み取れる
- `openapi.json` を利用したクライアントコード生成ツール（`openapi-generator` 等）の出力品質が向上する

**ネガティブ:**
- description の文字列がルーターコード中に増えるため、長文になる場合は可読性に注意が必要

## Implementation Notes

変更対象ファイル：

| ファイル | 変更内容 |
|---|---|
| `api/app/presentation/routers/users.py` | `create_user` / `get_user` / `deactivate_user` の `@router.*` デコレータに `summary` / `description` を追加 |
| `api/app/presentation/routers/posts.py` | `create_post` / `list_posts` / `get_post` の `@router.*` デコレータに `summary` / `description` を追加 |

テストへの影響：
- 既存テストのリクエスト/レスポンス構造に変更なし
- `/openapi.json` の内容を検証するテストが存在する場合は更新が必要（現時点では該当テストなし）
