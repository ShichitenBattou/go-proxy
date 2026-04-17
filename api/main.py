import logging
from collections.abc import AsyncGenerator
from contextlib import asynccontextmanager
from typing import TYPE_CHECKING

from app.presentation.execption_handler import add_exception_handler

if TYPE_CHECKING:
    pass

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app.config import settings
from app.infrastructure.jwks_client import JwksClient
from app.presentation import auth
from app.presentation.routers import posts, users
from app.setup.logging import configure_logging

configure_logging()
logger = logging.getLogger(__name__)


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncGenerator[None, None]:
    """アプリケーションのライフサイクル管理

    起動時に JWKS クライアントを初期化し、シャットダウン時に停止する。
    """
    # 起動時の処理
    logger.info("Starting up API server...")

    # JWKS クライアントを初期化
    jwks_client = JwksClient(
        jwks_uri=settings.keycloak_jwks_uri,
        cache_keys=True,
        refresh_interval=settings.jwks_refresh_interval,
    )

    jwks_client.start()

    # 認証設定をグローバルに設定
    auth.set_auth_config(
        jwks_client=jwks_client,
        issuer=settings.keycloak_issuer,
        audience=settings.keycloak_audience,
    )

    logger.info("JWKS client initialized and started")

    yield

    # シャットダウン時の処理
    logger.info("Shutting down API server...")
    await jwks_client.stop()
    logger.info("JWKS client stopped")


app = FastAPI(title="API", lifespan=lifespan)

origins = [
    "http://localhost:3000",
]

app.add_middleware(
    CORSMiddleware,
    allow_origins=origins,
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

add_exception_handler(app)

app.include_router(users.router)
app.include_router(posts.router)


@app.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok"}
