import asyncio
import logging
from typing import Any

from jwt import PyJWKClient, PyJWKClientError

from app.config import settings

logger = logging.getLogger(__name__)


class JwksClient:
    """JWKS (JSON Web Key Set) クライアント

    Keycloak の公開鍵をキャッシュし、JWT の検証に使用する。
    - unknown kid → 即時更新 + 1回リトライ
    - 定期バックグラウンド更新（デフォルト5分間隔）
    """

    def __init__(
        self,
        jwks_uri: str,
        cache_keys: bool = True,
        refresh_interval: int = 300,  # 5分
    ) -> None:
        self._jwks_uri = jwks_uri
        self._cache_keys = cache_keys
        self._refresh_interval = refresh_interval
        self._client: PyJWKClient | None = None
        self._refresh_task: asyncio.Task[None] | None = None

    def start(self) -> None:
        """JWKS クライアントを初期化し、定期更新タスクを開始"""
        self._client = PyJWKClient(
            uri=self._jwks_uri,
            cache_keys=self._cache_keys,
        )
        logger.info(f"JWKS client initialized with URI: {self._jwks_uri}")

        # 定期更新タスクを開始
        if settings.stage == "dev":
            pass
        else:
            self._refresh_task = asyncio.create_task(self._periodic_refresh())
            logger.info(f"JWKS periodic refresh task started (interval: {self._refresh_interval}s)")

    async def stop(self) -> None:
        """定期更新タスクを停止"""
        if self._refresh_task:
            self._refresh_task.cancel()
            try:
                await self._refresh_task
            except asyncio.CancelledError:
                pass
            logger.info("JWKS periodic refresh task stopped")

    def get_signing_key(self, kid: str) -> Any:
        """指定された kid の署名鍵を取得

        Args:
            kid: JWT ヘッダーの kid (Key ID)

        Returns:
            PyJWK 署名鍵

        Raises:
            PyJWKClientError: kid が見つからない、または JWKS 取得に失敗
        """
        if not self._client:
            raise RuntimeError("JWKS client not initialized. Call start() first.")

        try:
            return self._client.get_signing_key(kid)
        except PyJWKClientError as e:
            # unknown kid の場合、キャッシュをクリアして即時更新 + リトライ
            logger.warning(f"Unknown kid '{kid}', refreshing JWKS and retrying: {e}")
            self._force_refresh()
            # リトライ（2回目で失敗したら例外を上げる）
            return self._client.get_signing_key(kid)

    def _force_refresh(self) -> None:
        """JWKS を強制的に再取得（同期処理）"""
        if not self._client:
            return

        logger.info("Forcing JWKS refresh...")
        # PyJWKClient の内部キャッシュをクリア
        self._client._cached_keys = None  # type: ignore[attr-defined]
        # 次回 get_signing_key() で自動的に再取得される

    async def _periodic_refresh(self) -> None:
        """定期的に JWKS を更新するバックグラウンドタスク"""
        while True:
            try:
                await asyncio.sleep(self._refresh_interval)
                logger.debug("Running periodic JWKS refresh...")
                self._force_refresh()
            except asyncio.CancelledError:
                logger.debug("Periodic refresh task cancelled")
                raise
            except Exception as e:
                logger.error(f"Error during periodic JWKS refresh: {e}", exc_info=True)
