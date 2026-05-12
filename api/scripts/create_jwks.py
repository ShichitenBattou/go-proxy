#!/usr/bin/env python3
"""JWKS (JSON Web Key Set) 生成スクリプト

public-key.pem から jwks.json を生成して出力する。
鍵ペアを再生成した後に実行して jwks.json を更新する。

使い方:
    uv run scripts/create_jwks.py
    uv run scripts/create_jwks.py --kid my-key-id
    uv run scripts/create_jwks.py --public-key path/to/public-key.pem --out path/to/jwks.json
"""

import argparse
import base64
import json
from pathlib import Path

from cryptography.hazmat.primitives.asymmetric.rsa import RSAPublicKey
from cryptography.hazmat.primitives.serialization import load_pem_public_key

REPO_ROOT = Path(__file__).parent.parent
DEFAULT_PUBLIC_KEY_PATH = REPO_ROOT / "public-key.pem"
DEFAULT_OUTPUT_PATH = REPO_ROOT / "jwks.json"
DEFAULT_KID = "my-key-id"


def _b64url(n: int, byte_length: int) -> str:
    """整数を Base64URL エンコードする（パディングなし）"""
    return base64.urlsafe_b64encode(n.to_bytes(byte_length, "big")).rstrip(b"=").decode()


def build_jwks(public_key_path: Path, kid: str) -> dict[str, list[dict[str, str]]]:
    pem = public_key_path.read_bytes()
    public_key = load_pem_public_key(pem)

    if not isinstance(public_key, RSAPublicKey):
        raise ValueError(f"RSA 公開鍵が必要です: {type(public_key)}")

    pub_numbers = public_key.public_numbers()
    byte_length = (public_key.key_size + 7) // 8

    jwk = {
        "kty": "RSA",
        "kid": kid,
        "use": "sig",
        "alg": "RS256",
        "n": _b64url(pub_numbers.n, byte_length),
        "e": _b64url(pub_numbers.e, 3),
    }
    return {"keys": [jwk]}


def main() -> None:
    parser = argparse.ArgumentParser(description="public-key.pem から jwks.json を生成する")
    parser.add_argument(
        "--public-key",
        type=Path,
        default=DEFAULT_PUBLIC_KEY_PATH,
        help=f"公開鍵ファイルのパス（デフォルト: {DEFAULT_PUBLIC_KEY_PATH}）",
    )
    parser.add_argument(
        "--kid",
        default=DEFAULT_KID,
        help=f"Key ID（デフォルト: {DEFAULT_KID}）",
    )
    parser.add_argument(
        "--out",
        type=Path,
        default=None,
        help=f"出力先ファイルパス（省略時は標準出力。--save で {DEFAULT_OUTPUT_PATH} に保存）",
    )
    parser.add_argument(
        "--save",
        action="store_true",
        help=f"{DEFAULT_OUTPUT_PATH} に保存する",
    )
    args = parser.parse_args()

    jwks = build_jwks(args.public_key, args.kid)
    output = json.dumps(jwks, separators=(",", ":"))

    if args.out:
        args.out.write_text(output)
        print(f"保存しました: {args.out}")
    elif args.save:
        DEFAULT_OUTPUT_PATH.write_text(output)
        print(f"保存しました: {DEFAULT_OUTPUT_PATH}")
    else:
        print(output)


if __name__ == "__main__":
    main()
