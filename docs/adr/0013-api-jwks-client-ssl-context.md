# ADR-0013: PyJWKClient への ssl_context 設定

## Status
承認済み（Accepted） - 2026-04-30

## Context

現在の `JwksClient` は `PyJWKClient` を ssl_context なしで初期化している。

```python
self._client = PyJWKClient(
    uri=self._jwks_uri,
    cache_keys=self._cache_keys,
)
```

Keycloak が自己署名証明書や内部 CA 証明書を使用する環境（開発・ステージング）では、JWKS エンドポイントへの HTTPS 接続時に証明書検証エラーが発生しうる。また本番環境でも、特定の CA バンドルを明示的に指定したいケースがある。

`PyJWKClient` は `ssl_context` パラメータ（`ssl.SSLContext`）を受け付ける。このパラメータを使うことで、証明書検証の挙動を環境ごとに制御できる。

## Decision

以下の方針で `ssl_context` を `PyJWKClient` に渡す。

1. **設定値として `jwks_ssl_ca_bundle` を追加** — CA バンドルファイルのパスを環境変数 `JWKS_SSL_CA_BUNDLE` で受け取る（省略時は `None`）。型は `pathlib.Path | None` とし、Pydantic Settings の自動変換を利用する
2. **`ssl.SSLContext` を生成して渡す** — `jwks_ssl_ca_bundle` が指定されたときはそのファイルを CA として読み込む。未指定時は `ssl_context=None` のままとし、Python デフォルトの検証を使用する
3. **SSL 検証の無効化は提供しない** — 本プロジェクトは堅めの POC であり、`verify=False` 相当の設定は MITM 攻撃に脆弱になるため採用しない
4. **JWKS クライアントの組み立てを `app/setup/jwks.py` に集約** — `_build_ssl_context` と `JwksClient` インスタンス化を `build_jwks_client(settings)` として一箇所にまとめ、`main.py` から責務を分離する

```python
# app/setup/jwks.py
import ssl
from pathlib import Path

def build_jwks_client(settings: Settings) -> JwksClient:
    ssl_context = _build_ssl_context(settings.jwks_ssl_ca_bundle)
    return JwksClient(
        jwks_uri=settings.keycloak_jwks_uri,
        cache_keys=True,
        refresh_interval=settings.jwks_refresh_interval,
        ssl_context=ssl_context,
    )

def _build_ssl_context(ca_bundle: Path | None) -> ssl.SSLContext | None:
    if ca_bundle:
        return ssl.create_default_context(cafile=ca_bundle)
    return None
```

`JwksClient.__init__` のシグネチャ変更:

```python
def __init__(
    self,
    jwks_uri: str,
    cache_keys: bool = True,
    refresh_interval: int = 300,
    ssl_context: ssl.SSLContext | None = None,
) -> None:
```

`Settings` への追加:

```python
from pathlib import Path

jwks_ssl_ca_bundle: Path | None = None
```

`start()` での組み立て:

```python
self._client = PyJWKClient(
    uri=self._jwks_uri,
    cache_keys=self._cache_keys,
    ssl_context=self._ssl_context,
)
```

`main.py` からは `build_jwks_client(settings)` を呼ぶだけとし、SSL 構築の詳細を持たない。

## Consequences

**ポジティブ:**
- 自己署名証明書や内部 CA を使用する環境での JWKS 取得が可能になる
- SSL 検証の無効化を提供しないことで、セキュリティポリシーを維持できる
- `ssl_context=None`（デフォルト）はゼロ変更で既存挙動を維持する
- `app/setup/` に集約することで `main.py` は起動シーケンスのみに集中できる
- `Path` 型により OS パス操作の安全性が高まる

**ネガティブ:**
- SSL 検証を無効化したい場合（例: 開発環境での手軽な動作確認）は、CA バンドルを用意するか証明書を正しく設定する必要がある

## Implementation Notes

- `api/app/config.py`: `jwks_ssl_ca_bundle: Path | None = None` を追加
- `api/app/infrastructure/jwks_client.py`: `ssl_context: ssl.SSLContext | None` パラメータを追加し `PyJWKClient` に渡す
- `api/app/setup/jwks.py`: `build_jwks_client(settings)` と `_build_ssl_context(ca_bundle)` を新設
- `api/main.py`: `build_jwks_client(settings)` を呼ぶだけに変更し、SSL 構築ロジックを削除
- テスト: `ssl_context` が `PyJWKClient` に渡されることと `_build_ssl_context` の挙動を確認するユニットテストを追加
