# ADR-0012: API の JSON ログ出力の統一

## Status
提案中（Proposed） - 2026-04-29

## Context

現在 `api/app/setup/logging.py` には `pythonjsonlogger` による JSON フォーマッターが実装されているが、以下の問題がある。

### 問題 1: 本番環境（`stage != "dev"`）で stdout にログが出力されない

```python
else:
    logging.basicConfig(level=logging.INFO)
    fh = TimedRotatingFileHandler("api.log", ...)  # ファイルのみ
    fh.setFormatter(formatter)
    logging.getLogger().addHandler(fh)
    # stdout ハンドラーがない
```

Docker コンテナ環境では `docker logs` や Loki などのログ収集基盤が stdout を監視する。ファイルにのみ書き出すと、コンテナ外からログが観測できない。

### 問題 2: ファイルローテーションがコンテナ環境に不適

`TimedRotatingFileHandler` によるローカルファイルへの書き込みは、12ファクター・アプリの原則に反し、コンテナの stateless 設計と相容れない。ログの永続化はコンテナ外のインフラ（Loki, CloudWatch 等）が担うべきである。

### 問題 3: `basicConfig` とハンドラー追加の競合リスク

`logging.basicConfig` を呼んだあとにハンドラーを追加する現在の実装は、root logger に重複ハンドラーが追加されるリスクがある。

### 問題 4: uvicorn アクセスログが JSON 化されていない

uvicorn 自体の `access` / `error` ロガーは Python の標準 `logging` モジュールを使うが、現在の設定では JSON フォーマットが適用されない。

## Decision

### ログ出力先を stdout のみに統一する

`TimedRotatingFileHandler` を廃止し、`StreamHandler(sys.stdout)` のみを使用する。

### 環境によるレベル分岐は維持する

- `stage == "dev"`: `DEBUG`
- それ以外: `INFO`

### `basicConfig` を使わず、root logger を直接設定する

`basicConfig` の副作用を避けるため、root logger のレベルとハンドラーを明示的に設定する。

### uvicorn ロガーにも同じ JSON フォーマッターを適用する

`uvicorn.access` と `uvicorn.error` ロガーを取得し、同一の `JsonFormatter` でハンドラーを設定する。uvicorn 起動時に `--no-access-log` は使わない。

### 実装イメージ

```python
def configure_logging() -> None:
    level = logging.DEBUG if settings.stage == "dev" else logging.INFO

    formatter = JsonFormatter(
        "%(asctime)s %(levelname)s %(message)s %(pathname)s %(lineno)d %(process)d"
    )

    handler = logging.StreamHandler(sys.stdout)
    handler.setFormatter(formatter)

    root = logging.getLogger()
    root.setLevel(level)
    root.handlers.clear()
    root.addHandler(handler)

    for name in ("uvicorn", "uvicorn.access", "uvicorn.error"):
        uvicorn_logger = logging.getLogger(name)
        uvicorn_logger.handlers.clear()
        uvicorn_logger.propagate = True  # root に委譲
```

## Consequences

**ポジティブ:**
- ✅ 全ログが JSON 形式で stdout に統一され、Docker / Loki 等のログ収集基盤で即時利用可能
- ✅ コンテナ内にファイルが生成されなくなり、stateless 設計に準拠
- ✅ uvicorn アクセスログも JSON 化されるため、構造化ログによる検索・分析が可能
- ✅ ハンドラー設定がシンプルになり、重複追加のリスクが解消される

**ネガティブ:**
- ⚠️ ファイルローテーションがなくなるため、ログの長期保存は別途インフラ側で対応が必要

## Implementation Notes

- `api/app/setup/logging.py`: `TimedRotatingFileHandler` を削除し、stdout ハンドラーのみに変更
- テストへの影響: ログ設定はアプリ起動時のみ呼ばれるため、既存テストへの影響はない
