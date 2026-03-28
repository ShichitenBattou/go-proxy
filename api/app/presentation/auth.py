import json
import logging
import sys
from typing import Annotated

import jwt
from fastapi import Depends, HTTPException, status
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer
from jose import jwt as jose_jwt

from app.config import settings
from app.infrastructure.jwks_client import JwksClient

logger = logging.getLogger(__name__)

# FastAPI の HTTPBearer スキーム（Authorization: Bearer <token> を解析）
bearer_scheme = HTTPBearer(auto_error=False)


# グローバル変数として JwksClient インスタンスを保持
# main.py の lifespan で初期化される
_jwks_client: JwksClient | None = None
_issuer: str | None = None
_audience: str | None = None


def set_auth_config(jwks_client: JwksClient, issuer: str, audience: str) -> None:
    """認証設定をグローバルに設定（main.py から呼ばれる）

    Args:
        jwks_client: JWKS クライアントインスタンス
        issuer: Keycloak の issuer URL
        audience: クライアント ID（aud クレーム）
    """
    global _jwks_client, _issuer, _audience
    _jwks_client = jwks_client
    _issuer = issuer
    _audience = audience
    logger.info(f"Auth config set: issuer={issuer}, audience={audience}")


async def get_current_sub(
    credentials: Annotated[HTTPAuthorizationCredentials | None, Depends(bearer_scheme)],
) -> str:
    """Authorization ヘッダーから JWT を検証し、sub クレームを返す

    Args:
        credentials: HTTPBearer スキームから取得した認証情報

    Returns:
        JWT の sub クレーム（Keycloak のユーザー ID）

    Raises:
        HTTPException: トークンが無効、または必須クレームが欠けている場合
    """
    if not credentials:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Missing Authorization header",
            headers={"WWW-Authenticate": "Bearer"},
        )

    if not _jwks_client or not _issuer or not _audience:
        logger.error("Auth config not initialized")
        logger.debug(f"_jwks_client: {_jwks_client}, _issuer: {_issuer}, _audience: {_audience}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail="Authentication not configured",
        )

    token = credentials.credentials

    try:
        # JWT ヘッダーから kid を取得（署名検証前）
        unverified_header = jwt.get_unverified_header(token)
        kid = unverified_header.get("kid")
        if not kid:
            raise HTTPException(
                status_code=status.HTTP_401_UNAUTHORIZED,
                detail="Missing 'kid' in JWT header",
                headers={"WWW-Authenticate": "Bearer"},
            )

        if settings.stage == "dev":
            with open("jwks.json") as file:
                jwks_data = json.load(file)

            signing_key = jwt.PyJWK(next(x for x in jwks_data["keys"] if x["kid"] == kid)).key
        else:
            # JWKS から署名鍵を取得
            signing_key = _jwks_client.get_signing_key(kid).key

        json.dump(
            jwt.get_unverified_header(token), sys.stdout, indent=2
        )  # ヘッダーの検証（kid の存在確認など）
        json.dump(
            jose_jwt.get_unverified_claims(token),  # クレームの検証（iss, aud の存在確認など）
            sys.stdout,
            indent=2,
        )

        print(
            f"issuer: {_issuer}, token issuer: {jose_jwt.get_unverified_claims(token).get('iss')}"
        )

        # JWT をデコード・検証
        payload = jwt.decode(
            token,
            signing_key,
            algorithms=["RS256"],
            issuer=_issuer,
            audience=_audience,
            options={
                "verify_signature": True,
                "verify_exp": True,
                "verify_iss": True,
                "verify_aud": True,
            },
        )

        # sub クレームを取得
        sub = payload.get("sub")
        if not sub:
            raise HTTPException(
                status_code=status.HTTP_401_UNAUTHORIZED,
                detail="Missing 'sub' claim in JWT",
                headers={"WWW-Authenticate": "Bearer"},
            )

        logger.debug(f"JWT verified successfully for sub: {sub}")
        return str(sub)

    except jwt.ExpiredSignatureError:
        logger.warning("JWT expired")
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Token has expired",
            headers={"WWW-Authenticate": "Bearer"},
        )
    except jwt.InvalidIssuerError:
        logger.warning(f"Invalid issuer (expected: {_issuer})")
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid token issuer",
            headers={"WWW-Authenticate": "Bearer"},
        )
    except jwt.InvalidAudienceError:
        logger.warning(f"Invalid audience (expected: {_audience})")
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid token audience",
            headers={"WWW-Authenticate": "Bearer"},
        )
    except jwt.InvalidTokenError as e:
        logger.warning(f"Invalid JWT: {e}")
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail=f"Invalid token: {str(e)}",
            headers={"WWW-Authenticate": "Bearer"},
        )
    except Exception as e:
        logger.error(f"Unexpected error during JWT verification: {e}", exc_info=True)
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail="Authentication failed",
        )


# 型エイリアス（FastAPI の Depends で使用）
CurrentSub = Annotated[str, Depends(get_current_sub)]
