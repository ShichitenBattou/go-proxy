# ADR-0012: API ログの JSON 出力への完全統一

## Status
提案中（Proposed） - 2026-04-29

## Context

`api/app/setup/logging.py` には `pythonjsonlogger` の `JsonFormatter` が設定されているが、以下の2つの問題により全ログが JSON にならない。

### 問題 1: `basicConfig` によるテキストハンドラーの混入

```python
logging.basicConfig(level=logging.DEBUG)  # デフォルトのテキストハンドラーを追加
sh = logging.StreamHandler(sys.stdout)
sh.setFormatter(formatter)  # JSON ハンドラーを追加
logging.getLogger().addHandler(sh)
```

`basicConfig` はルートロガーにデフォルト（テキスト）ハンドラーを追加する。その後に JSON ハンドラーを追加するため、**両方が動作し**、同一ログが2回出力される（テキスト形式 + JSON 形式）。

実際に観測されたテキスト形式の出力:
```
ERROR:app.presentation.auth:Unexpected error during JWT verification: ...
```

### 問題 2: uvicorn アクセスログが JSON 化されていない

uvicorn 自体の `uvicorn.access` / `uvicorn.error` ロガーは独自ハンドラーを持つため、root logger の JSON 設定が適用されない。

## Decision

`configure_logging()` を以下の方針で書き直す。

1. **`basicConfig` を廃止** — root logger のレベルとハンドラーを直接設定する
2. **`root.handlers.clear()` で既存ハンドラーを除去** — 重複出力を防ぐ
3. **JSON stdout ハンドラーのみ設定** — テキストハンドラーを追加しない
4. **uvicorn ロガーを root に委譲** — `handlers.clear()` + `propagate = True`

```python
def configure_logging() -> None:
    level = logging.DEBUG if settings.stage == "dev" else logging.INFO

    formatter = JsonFormatter(
        "%(asctime)s - %(levelname)s - %(message)s - %(pathname)s - %(lineno)d - %(process)d"
    )

    handler = logging.StreamHandler(sys.stdout)
    handler.setLevel(level)
    handler.setFormatter(formatter)

    root = logging.getLogger()
    root.setLevel(level)
    root.handlers.clear()
    root.addHandler(handler)

    for name in ("uvicorn", "uvicorn.access", "uvicorn.error"):
        uvicorn_logger = logging.getLogger(name)
        uvicorn_logger.handlers.clear()
        uvicorn_logger.propagate = True
```

## Consequences

**ポジティブ:**
- 全ログ（アプリ・uvicorn アクセスログ）が JSON 形式に統一される
- 同一ログの二重出力が解消される
- ファイルハンドラーを削除し stdout のみにすることでコンテナ設計に準拠

**ネガティブ:**
- ファイルへのローテーション出力がなくなる（ログ永続化はインフラ側で対応）

## Implementation Notes

- `api/app/setup/logging.py`: `configure_logging()` を全面的に書き直す
- テストへの影響なし
