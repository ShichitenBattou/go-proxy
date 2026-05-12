import ssl
from pathlib import Path
from unittest.mock import MagicMock, patch

from app.infrastructure.jwks_client import JwksClient
from app.setup.jwks import _build_ssl_context, build_jwks_client


class TestJwksClientSslContext:
    def test_start_passes_none_ssl_context_by_default(self) -> None:
        client = JwksClient(jwks_uri="https://example.com/jwks")

        with (
            patch("app.infrastructure.jwks_client.PyJWKClient") as mock_cls,
            patch("app.infrastructure.jwks_client.settings") as mock_settings,
        ):
            mock_settings.stage = "dev"  # create_task を回避
            client.start()

        mock_cls.assert_called_once_with(
            uri="https://example.com/jwks",
            cache_keys=True,
            ssl_context=None,
        )

    def test_start_passes_given_ssl_context(self) -> None:
        ctx = ssl.create_default_context()
        client = JwksClient(jwks_uri="https://example.com/jwks", ssl_context=ctx)

        with (
            patch("app.infrastructure.jwks_client.PyJWKClient") as mock_cls,
            patch("app.infrastructure.jwks_client.settings") as mock_settings,
        ):
            mock_settings.stage = "dev"  # create_task を回避
            client.start()

        mock_cls.assert_called_once_with(
            uri="https://example.com/jwks",
            cache_keys=True,
            ssl_context=ctx,
        )


class TestBuildSslContext:
    def test_returns_none_when_ca_bundle_is_none(self) -> None:
        result = _build_ssl_context(None)

        assert result is None

    def test_returns_ssl_context_when_ca_bundle_is_given(self) -> None:
        ca_bundle = Path("/path/to/ca.pem")
        with patch("ssl.create_default_context") as mock_ctx:
            mock_ctx.return_value = MagicMock(spec=ssl.SSLContext)
            result = _build_ssl_context(ca_bundle)

        mock_ctx.assert_called_once_with(cafile=ca_bundle)
        assert result is not None


class TestBuildJwksClient:
    def test_builds_client_with_ssl_context(self) -> None:
        mock_settings = MagicMock()
        mock_settings.keycloak_jwks_uri = "https://example.com/jwks"
        mock_settings.jwks_refresh_interval = 300
        mock_settings.jwks_ssl_ca_bundle = Path("/path/to/ca.pem")

        ctx = MagicMock(spec=ssl.SSLContext)
        with patch("app.setup.jwks._build_ssl_context", return_value=ctx) as mock_build:
            client = build_jwks_client(mock_settings)

        mock_build.assert_called_once_with(mock_settings.jwks_ssl_ca_bundle)
        assert isinstance(client, JwksClient)

    def test_builds_client_without_ssl_context(self) -> None:
        mock_settings = MagicMock()
        mock_settings.keycloak_jwks_uri = "https://example.com/jwks"
        mock_settings.jwks_refresh_interval = 300
        mock_settings.jwks_ssl_ca_bundle = None

        with patch("app.setup.jwks._build_ssl_context", return_value=None) as mock_build:
            client = build_jwks_client(mock_settings)

        mock_build.assert_called_once_with(None)
        assert isinstance(client, JwksClient)
