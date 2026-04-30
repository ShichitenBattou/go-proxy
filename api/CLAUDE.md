# CLAUDE.md

このファイルは、リポジトリで作業する Claude Code (claude.ai/code) へのガイダンスを提供します。

## 概要

API バックエンドサービス — BFF レイヤーからプロキシされたリクエストを受け取る Python/FastAPI HTTP サーバー。
クリーンアーキテクチャ ([参考](https://github.com/ivan-borovets/fastapi-clean-example)) を採用。Docker 内ではポート 8000 で動作する。

## ルール（厳守）

- 実装時、テストコードを必ず追加すること

## コマンド

```bash
# 依存関係のインストール
uv sync

# ローカル起動
task run

# テスト実行
uv run pytest

# 依存パッケージの追加
uv add <package>
```

### Alembic マイグレーション

DB への接続が必要なため、`task` コマンド経由で Docker Compose を使って実行する。

```bash
# マイグレーションファイルの自動生成
task revision -- MSG="説明"

# マイグレーション適用
task migrate

# 任意の alembic コマンド実行 (例: downgrade)
task alembic -- CLI_ARGS="downgrade -1"
```

## アーキテクチャ

クリーンアーキテクチャ + CQRS を採用。依存方向は内側に向かう一方通行:

```
presentation ──→ application ──→ domain
infrastructure ──→ application ──→ domain
```

```
app/
├── domain/          # ビジネスルールの核心。外部依存ゼロ
│   ├── entities/    # User, Post (dataclass)
│   ├── ports/       # リポジトリインターフェース (typing.Protocol)
│   └── exceptions/  # ドメイン例外
├── application/     # ユースケース。1ユースケース = 1 Interactor クラス
│   ├── commands/    # 書き込み操作 (CreateUser, CreatePost)
│   └── queries/     # 読み取り操作 (GetUser, ListPosts)
├── infrastructure/  # ポートの具体実装 (SQLAlchemy, asyncpg)
│   ├── database.py  # engine, AsyncSessionLocal, Base, get_db()
│   ├── models/      # SQLAlchemy ORM モデル (UserModel, PostModel)
│   └── repositories/# SqlAlchemy* リポジトリ実装
└── presentation/    # 薄い HTTP レイヤー
    ├── dependencies.py  # FastAPI Depends でインタラクターを組み立て
    └── routers/     # FastAPI ルーター + Pydantic スキーマ
```

### DI パターン

Dishka は使用しない。FastAPI の `Depends` でリポジトリ → インタラクターを組み立てる。
`presentation/dependencies.py` が配線の中心。

### ドメインモデル

- **User**: `id` (UUID), `keycloak_sub` (Keycloak の sub クレーム・一意), `role` (admin/user), `is_active`
- **Post**: `id`, `author_id`, `title`, `body`, `tags` (文字列配列), `created_at`, `version`
- 投稿可能なのは `role=USER` かつ `is_active=True` のユーザーのみ (管理者は投稿不可)

### トランザクション管理

`get_db()` がリクエスト終了時に `commit`、例外時に `rollback` する。
各リポジトリメソッドは `flush()` を呼ぶが `commit()` しない。

## 技術スタック

- Python 3.12 (`.python-version` でピン留め)
- FastAPI + uvicorn
- SQLAlchemy 2.0 (async) + asyncpg
- Alembic (async 対応 `env.py`)
- Docker イメージ: `python:3.14.3`、`uvicorn main:app` を `0.0.0.0:8000` で起動
