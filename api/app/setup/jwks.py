import ssl
from pathlib import Path

from app.config import Settings
from app.infrastructure.jwks_client import JwksClient


def build_jwks_client(settings: Settings) -> JwksClient:
    """設定から JwksClient を組み立てる。

    Args:
        settings: アプリケーション設定

    Returns:
        JwksClient インスタンス（未起動）
    """
    return JwksClient(
        jwks_uri=settings.keycloak_jwks_uri,
        cache_keys=True,
        refresh_interval=settings.jwks_refresh_interval,
        ssl_context=_build_ssl_context(settings.jwks_ssl_ca_bundle),
    )


def _build_ssl_context(ca_bundle: Path | None) -> ssl.SSLContext | None:
    """JWKS 取得用の SSLContext を生成する。

    Args:
        ca_bundle: CA バンドルファイルのパス。None のとき Python デフォルトの検証を使用。

    Returns:
        SSLContext。ca_bundle 未指定時は None。
    """
    if ca_bundle:
        return ssl.create_default_context(cafile=ca_bundle)
    return None
