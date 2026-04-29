# ADR-0013: PyJWKClient への ssl_context 設定

## Status
提案中（Proposed） - 2026-04-29

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

1. **設定値として `jwks_ssl_ca_bundle` を追加** — CA バンドルファイルのパスを環境変数 `JWKS_SSL_CA_BUNDLE` で受け取る（省略時は `None`）
2. **設定値として `jwks_ssl_verify` を追加** — SSL 検証を無効にするフラグ（`bool`、デフォルト `True`）
3. **`ssl.SSLContext` を生成して渡す** — `jwks_ssl_verify=False` のとき検証を無効化、`jwks_ssl_ca_bundle` が指定されたときはそのファイルを CA として読み込む
4. **`ssl_context=None` は変更なし** — 両設定がデフォルト値のとき（`verify=True` かつ CA バンドル未指定）は `ssl_context=None` のままとし、Python デフォルトの検証を使用する

```python
import ssl

def _build_ssl_context(verify: bool, ca_bundle: str | None) -> ssl.SSLContext | None:
    if not verify:
        ctx = ssl.create_default_context()
        ctx.check_hostname = False
        ctx.verify_mode = ssl.CERT_NONE
        return ctx
    if ca_bundle:
        ctx = ssl.create_default_context(cafile=ca_bundle)
        return ctx
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
jwks_ssl_verify: bool = True
jwks_ssl_ca_bundle: str | None = None
```

`start()` での組み立て:

```python
self._client = PyJWKClient(
    uri=self._jwks_uri,
    cache_keys=self._cache_keys,
    ssl_context=self._ssl_context,
)
```

`JwksClient` の組み立ては `presentation/dependencies.py` で行い、`settings` から `ssl_context` を生成して渡す。

## Consequences

**ポジティブ:**
- 自己署名証明書環境での JWKS 取得が可能になる
- 本番環境で特定の CA バンドルを明示指定できる
- `ssl_context=None`（デフォルト）はゼロ変更で既存挙動を維持する

**ネガティブ:**
- `jwks_ssl_verify=False` は MITM 攻撃に脆弱になるため、開発環境に限定すること

## Implementation Notes

- `api/app/config.py`: `jwks_ssl_verify: bool = True`, `jwks_ssl_ca_bundle: str | None = None` を追加
- `api/app/infrastructure/jwks_client.py`: `ssl_context: ssl.SSLContext | None` パラメータを追加し `PyJWKClient` に渡す
- `api/app/presentation/dependencies.py`: `_build_ssl_context()` ヘルパーを追加し `JwksClient` を組み立て
- テスト: `ssl_context` が `PyJWKClient` に渡されることを確認するユニットテストを追加
