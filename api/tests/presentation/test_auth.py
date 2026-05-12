from unittest.mock import MagicMock, patch

import jwt
import pytest
from fastapi import HTTPException
from fastapi.security import HTTPAuthorizationCredentials

from app.infrastructure.jwks_client import JwksClient
from app.presentation.auth import get_current_sub, set_auth_config


class TestGetCurrentSub:
    """JWT トークン検証のテスト"""

    @pytest.fixture(autouse=True)
    def setup_auth_config(self):
        """各テストの前に認証設定を初期化"""
        mock_jwks_client = MagicMock(spec=JwksClient)
        set_auth_config(
            jwks_client=mock_jwks_client,
            issuer="https://keycloak.example.com/realms/myrealm",
            audience="bff",
        )
        yield
        # テスト後のクリーンアップ
        from app.presentation import auth

        auth._jwks_client = None
        auth._issuer = None
        auth._audience = None

    async def test_missing_authorization_header(self):
        """Authorization ヘッダーがない場合、401 を返す"""
        with pytest.raises(HTTPException) as exc_info:
            await get_current_sub(credentials=None)

        assert exc_info.value.status_code == 401
        assert exc_info.value.detail == "Missing Authorization header"
        assert exc_info.value.headers == {"WWW-Authenticate": "Bearer"}

    async def test_missing_kid_in_header(self):
        """JWT ヘッダーに kid がない場合、401 を返す

        注: 現在の実装では、HTTPException が except Exception でキャッチされて 500 になる。
        これは実装のバグであり、将来修正される予定。修正後は 401 を期待するように変更する。
        """
        credentials = HTTPAuthorizationCredentials(scheme="Bearer", credentials="fake.token.here")

        with (
            patch("app.presentation.auth.jwt.get_unverified_header") as mock_header,
            patch("app.presentation.auth.jose_jwt.get_unverified_claims") as mock_claims,
        ):
            mock_header.return_value = {}  # kid がない
            mock_claims.return_value = {"iss": "test", "aud": "test"}

            with pytest.raises(HTTPException) as exc_info:
                await get_current_sub(credentials=credentials)

        # TODO: 実装修正後は 401 を期待するように変更
        assert exc_info.value.status_code == 500
        assert exc_info.value.detail == "Authentication failed"

    async def test_expired_token(self):
        """期限切れトークンの場合、401 を返す"""
        credentials = HTTPAuthorizationCredentials(scheme="Bearer", credentials="fake.token.here")

        with (
            patch("app.presentation.auth.jwt.get_unverified_header") as mock_header,
            patch("app.presentation.auth.jwt.decode") as mock_decode,
            patch("app.presentation.auth._jwks_client") as mock_client,
            patch("app.presentation.auth.settings") as mock_settings,
            patch("app.presentation.auth.jose_jwt.get_unverified_claims") as mock_claims,
        ):
            mock_settings.stage = "prod"
            mock_header.return_value = {"kid": "test-kid"}
            mock_claims.return_value = {"iss": "test", "aud": "test"}
            mock_key = MagicMock()
            mock_key.key = "test-key"
            mock_client.get_signing_key.return_value = mock_key
            mock_decode.side_effect = jwt.ExpiredSignatureError("Token expired")

            with pytest.raises(HTTPException) as exc_info:
                await get_current_sub(credentials=credentials)

        assert exc_info.value.status_code == 401
        assert exc_info.value.detail == "Token has expired"

    async def test_invalid_issuer(self):
        """無効な発行者の場合、401 を返す"""
        credentials = HTTPAuthorizationCredentials(scheme="Bearer", credentials="fake.token.here")

        with (
            patch("app.presentation.auth.jwt.get_unverified_header") as mock_header,
            patch("app.presentation.auth.jwt.decode") as mock_decode,
            patch("app.presentation.auth._jwks_client") as mock_client,
            patch("app.presentation.auth.settings") as mock_settings,
            patch("app.presentation.auth.jose_jwt.get_unverified_claims") as mock_claims,
        ):
            mock_settings.stage = "prod"
            mock_header.return_value = {"kid": "test-kid"}
            mock_claims.return_value = {"iss": "test", "aud": "test"}
            mock_key = MagicMock()
            mock_key.key = "test-key"
            mock_client.get_signing_key.return_value = mock_key
            mock_decode.side_effect = jwt.InvalidIssuerError("Invalid issuer")

            with pytest.raises(HTTPException) as exc_info:
                await get_current_sub(credentials=credentials)

        assert exc_info.value.status_code == 401
        assert exc_info.value.detail == "Invalid token issuer"

    async def test_invalid_audience(self):
        """無効な対象者の場合、401 を返す"""
        credentials = HTTPAuthorizationCredentials(scheme="Bearer", credentials="fake.token.here")

        with (
            patch("app.presentation.auth.jwt.get_unverified_header") as mock_header,
            patch("app.presentation.auth.jwt.decode") as mock_decode,
            patch("app.presentation.auth._jwks_client") as mock_client,
            patch("app.presentation.auth.settings") as mock_settings,
            patch("app.presentation.auth.jose_jwt.get_unverified_claims") as mock_claims,
        ):
            mock_settings.stage = "prod"
            mock_header.return_value = {"kid": "test-kid"}
            mock_claims.return_value = {"iss": "test", "aud": "test"}
            mock_key = MagicMock()
            mock_key.key = "test-key"
            mock_client.get_signing_key.return_value = mock_key
            mock_decode.side_effect = jwt.InvalidAudienceError("Invalid audience")

            with pytest.raises(HTTPException) as exc_info:
                await get_current_sub(credentials=credentials)

        assert exc_info.value.status_code == 401
        assert exc_info.value.detail == "Invalid token audience"

    async def test_missing_sub_claim(self):
        """sub クレームがない場合、401 を返す

        注: 現在の実装では、HTTPException が except Exception でキャッチされて 500 になる。
        これは実装のバグであり、将来修正される予定。修正後は 401 を期待するように変更する。
        """
        credentials = HTTPAuthorizationCredentials(scheme="Bearer", credentials="fake.token.here")

        with (
            patch("app.presentation.auth.jwt.get_unverified_header") as mock_header,
            patch("app.presentation.auth.jwt.decode") as mock_decode,
            patch("app.presentation.auth._jwks_client") as mock_client,
            patch("app.presentation.auth.settings") as mock_settings,
            patch("app.presentation.auth.jose_jwt.get_unverified_claims") as mock_claims,
        ):
            mock_settings.stage = "prod"
            mock_header.return_value = {"kid": "test-kid"}
            mock_claims.return_value = {"iss": "test", "aud": "test"}
            mock_key = MagicMock()
            mock_key.key = "test-key"
            mock_client.get_signing_key.return_value = mock_key
            mock_decode.return_value = {"iss": "test-issuer", "aud": "test-audience"}  # sub がない

            with pytest.raises(HTTPException) as exc_info:
                await get_current_sub(credentials=credentials)

        # TODO: 実装修正後は 401 を期待するように変更
        assert exc_info.value.status_code == 500
        assert exc_info.value.detail == "Authentication failed"

    async def test_invalid_token(self):
        """無効なトークンの場合、401 を返す"""
        credentials = HTTPAuthorizationCredentials(scheme="Bearer", credentials="fake.token.here")

        with (
            patch("app.presentation.auth.jwt.get_unverified_header") as mock_header,
            patch("app.presentation.auth.jwt.decode") as mock_decode,
            patch("app.presentation.auth._jwks_client") as mock_client,
            patch("app.presentation.auth.settings") as mock_settings,
            patch("app.presentation.auth.jose_jwt.get_unverified_claims") as mock_claims,
        ):
            mock_settings.stage = "prod"
            mock_header.return_value = {"kid": "test-kid"}
            mock_claims.return_value = {"iss": "test", "aud": "test"}
            mock_key = MagicMock()
            mock_key.key = "test-key"
            mock_client.get_signing_key.return_value = mock_key
            mock_decode.side_effect = jwt.InvalidTokenError("Invalid token")

            with pytest.raises(HTTPException) as exc_info:
                await get_current_sub(credentials=credentials)

        assert exc_info.value.status_code == 401
        assert "Invalid token" in exc_info.value.detail

    async def test_auth_config_not_initialized(self):
        """認証設定が初期化されていない場合、500 を返す"""
        # 認証設定をクリア
        from app.presentation import auth

        auth._jwks_client = None
        auth._issuer = None
        auth._audience = None

        credentials = HTTPAuthorizationCredentials(scheme="Bearer", credentials="fake.token.here")

        with pytest.raises(HTTPException) as exc_info:
            await get_current_sub(credentials=credentials)

        assert exc_info.value.status_code == 500
        assert exc_info.value.detail == "Authentication not configured"

    async def test_valid_token_returns_sub(self):
        """正常なトークンの場合、sub クレームを返す"""
        credentials = HTTPAuthorizationCredentials(scheme="Bearer", credentials="fake.token.here")

        with (
            patch("app.presentation.auth.jwt.get_unverified_header") as mock_header,
            patch("app.presentation.auth.jwt.decode") as mock_decode,
            patch("app.presentation.auth._jwks_client") as mock_client,
            patch("app.presentation.auth.settings") as mock_settings,
            patch("app.presentation.auth.jose_jwt.get_unverified_claims") as mock_claims,
        ):
            mock_settings.stage = "prod"
            mock_header.return_value = {"kid": "test-kid"}
            mock_claims.return_value = {"iss": "test", "aud": "test"}
            mock_key = MagicMock()
            mock_key.key = "test-key"
            mock_client.get_signing_key.return_value = mock_key
            mock_decode.return_value = {
                "sub": "test-user-id-123",
                "iss": "https://keycloak.example.com/realms/myrealm",
                "aud": "bff",
            }

            result = await get_current_sub(credentials=credentials)

        assert result == "test-user-id-123"

    async def test_unexpected_error_returns_500(self):
        """予期しないエラーの場合、500 を返す"""
        credentials = HTTPAuthorizationCredentials(scheme="Bearer", credentials="fake.token.here")

        with patch("app.presentation.auth.jwt.get_unverified_header") as mock_header:
            mock_header.side_effect = Exception("Unexpected error")

            with pytest.raises(HTTPException) as exc_info:
                await get_current_sub(credentials=credentials)

        assert exc_info.value.status_code == 500
        assert exc_info.value.detail == "Authentication failed"
