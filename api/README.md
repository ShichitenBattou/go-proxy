# API

FastAPI + SQLAlchemy kˆ‹ API ĞÃ¯¨óÉµüÓ¹s0o `CLAUDE.md` ’ÂgWfO`UD

## »ÃÈ¢Ã×

1. X¢Ân¤ó¹Èüë:
```bash
uv sync
```

2. °ƒ	pn-š:
```bash
cp .env.example .env
# .env Õ¡¤ë’èÆWfKeycloak n-š’LF
```

Åj°ƒ	p:
- `KEYCLOAK_ISSUER`: Keycloak n issuer URL (‹: `http://keycloak:8082/realms/myrealm`)
- `KEYCLOAK_AUDIENCE`: ¯é¤¢óÈ ID (‹: `bff`)
- `KEYCLOAK_JWKS_URI`: JWKS ¨óÉİ¤óÈ URL (‹: `http://keycloak:8082/realms/myrealm/protocol/openid-connect/certs`)

## <

API o Bearer JWT <’(W~Y

- `POST /posts/` ¨óÉİ¤óÈo<LÅgY
- Authorization ØÃÀük Bearer Èü¯ó’ØWfO`UD: `Authorization: Bearer <token>`
- Èü¯óo Keycloak K‰Ö—W_ JWT ’(W~Y
- •?n `author_id` o JWT n `sub` ¯ìüàK‰êÕ„k-šUŒ~Y

*{2næü¶üL•?’\WˆFhY‹h401 ¨éüLÔUŒ~Y‹Mk `POST /users/` gæü¶ü{2’LcfO`UD
