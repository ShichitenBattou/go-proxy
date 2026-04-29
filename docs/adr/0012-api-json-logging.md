# ADR-0012: uvicorn アクセスログの JSON 化

## Status
提案中（Proposed） - 2026-04-29

## Context

`api/app/setup/logging.py` にはアプリケーションログ向けに `pythonjsonlogger` の `JsonFormatter` が設定されており、JSON 出力が機能している。

しかし uvicorn 自体の `uvicorn.access` / `uvicorn.error` ロガーは独自のハンドラーを持つため、この設定が適用されず、テキスト形式で出力される。

```
INFO:     127.0.0.1:54321 - "GET /health HTTP/1.1" 200 OK
```

アプリログが JSON なのにアクセスログがテキストでは、ログ収集・検索の一貫性が損なわれる。

## Decision

`configure_logging()` 内で uvicorn ロガーのハンドラーをクリアし、`propagate = True` にして root logger の JSON ハンドラーに委譲する。

```python
for name in ("uvicorn", "uvicorn.access", "uvicorn.error"):
    uvicorn_logger = logging.getLogger(name)
    uvicorn_logger.handlers.clear()
    uvicorn_logger.propagate = True
```

既存のアプリケーションログの設定（`StreamHandler` + `JsonFormatter`、環境別レベル分岐）はそのまま維持する。

## Consequences

**ポジティブ:**
- アクセスログがアプリログと同じ JSON 形式に統一される
- 変更箇所が最小限（`logging.py` へのループ追加のみ）

**ネガティブ:**
- 特になし

## Implementation Notes

- `api/app/setup/logging.py`: `configure_logging()` の末尾に uvicorn ロガーの設定を追加
- テストへの影響なし
