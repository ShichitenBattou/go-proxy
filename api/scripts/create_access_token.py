#!/usr/bin/env python3
"""開発用アクセストークン生成スクリプト

private-key.pem で署名した JWT を生成して標準出力に表示する。
dev 環境での API 動作確認用。

使い方:
    uv run scripts/create_access_token.py
    uv run scripts/create_access_token.py --sub <keycloak-user-uuid>
    uv run scripts/create_access_token.py --sub <uuid> --expires-in 3600
"""

import argparse
import time
import uuid
from pathlib import Path

import jwt

PRIVATE_KEY_PATH = Path(__file__).parent.parent / "private-key.pem"

# jwks.json の kid と合わせる
KID = "my-key-id"

# .env.example のデフォルト値
DEFAULT_ISSUER = "http://keycloak:8082/realms/myrealm"
DEFAULT_AUDIENCE = "bff"
DEFAULT_EXPIRES_IN = 3600  # 1 時間


def create_token(
    sub: str,
    issuer: str,
    audience: str,
    expires_in: int,
) -> str:
    private_key = PRIVATE_KEY_PATH.read_text()

    now = int(time.time())
    payload = {
        "sub": sub,
        "iss": issuer,
        "aud": audience,
        "iat": now,
        "exp": now + expires_in,
    }
    headers = {"kid": KID}

    return jwt.encode(payload, private_key, algorithm="RS256", headers=headers)


def main() -> None:
    parser = argparse.ArgumentParser(description="開発用アクセストークンを生成する")
    parser.add_argument(
        "--sub",
        default=str(uuid.uuid4()),
        help="JWT の sub クレーム（Keycloak ユーザー UUID）。省略時はランダム UUID。",
    )
    parser.add_argument(
        "--issuer",
        default=DEFAULT_ISSUER,
        help=f"JWT の iss クレーム（デフォルト: {DEFAULT_ISSUER}）",
    )
    parser.add_argument(
        "--audience",
        default=DEFAULT_AUDIENCE,
        help=f"JWT の aud クレーム（デフォルト: {DEFAULT_AUDIENCE}）",
    )
    parser.add_argument(
        "--expires-in",
        type=int,
        default=DEFAULT_EXPIRES_IN,
        help=f"有効期限（秒）。デフォルト: {DEFAULT_EXPIRES_IN}",
    )
    args = parser.parse_args()

    token = create_token(
        sub=args.sub,
        issuer=args.issuer,
        audience=args.audience,
        expires_in=args.expires_in,
    )

    print(token)


if __name__ == "__main__":
    main()
