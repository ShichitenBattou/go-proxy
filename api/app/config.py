from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """アプリケーション設定

    環境変数から設定を読み込む。.env ファイルもサポート。
    """

    # Keycloak 認証設定
    keycloak_issuer: str
    keycloak_audience: str
    keycloak_jwks_uri: str

    # JWKS キャッシュ設定
    jwks_refresh_interval: int = 300  # 5分

    stage: str = "dev"  # 開発環境用のデフォルト値

    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
        extra="ignore",  # 未定義の環境変数を無視
    )


# シングルトンインスタンス
settings = Settings()  # type: ignore[call-arg]
