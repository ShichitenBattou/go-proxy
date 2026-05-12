# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Rules

- Work as a Go professional
- No assumptions - always ask for clarification when unclear

## Architecture Overview

This is a **Backend for Frontend (BFF)** authentication proxy written in Go. It sits between NGINX and the Python FastAPI backend, implementing session-based authentication with Keycloak OIDC integration.

### Core Responsibilities

1. **Session Management** — Validates session cookies on every request, stores sessions in Redis with SHA-256 hashed keys
2. **Reverse Proxy** — Strips `/api` prefix and forwards requests to the backend API
3. **Token Refresh** — On API 401 responses, refreshes the access token via refresh token and retries the request (ADR-0020)
4. **OIDC Authentication** — Redirects unauthenticated users to Keycloak, handles OAuth2 callbacks

### Request Flow

```
NGINX → BFF (:8080) → API (api:8081)
           ↕
        Redis (session:6379)
```

**On each API request:**
1. Validate `Session-Id` cookie against Redis
2. Strip `/api` prefix (e.g., `/api/users` → `/users`)
3. Forward to backend with `Authorization: Bearer <access_token>` and `Request-Id` headers
4. If API returns 401, refresh the access token and retry once (ADR-0020)

### Vertical Slice Architecture

The codebase follows **Vertical Slice Architecture** — features are organized by use case, not by technical layer:

- **`auth/`** — OIDC login/logout flow (`/auth/login`, `/auth/callback`, `/auth/me`, `/auth/logout`)
  - `LoginHandler` — Creates state token, stores in Redis, redirects to Keycloak
  - `NewCallbackHandler` — Exchanges authorization code for tokens, verifies ID token, runs JIT user provisioning, creates session, sets cookie
  - `MeHandler` — Returns current session user info
  - `LogoutHandler` — Deletes session from Redis, clears cookie

- **`proxy/`** — Reverse proxy handlers
  - `NewHandler` — `/api/*` proxy with session validation, path rewriting (removes `/api` prefix), token refresh on 401
  - `NewPublicHandler` — `/public/*` proxy without session validation (public endpoints)

- **`redis/`** — Session persistence layer (shared infrastructure)
  - `SetSession` / `GetSessionValue` / `DeleteSession` — Session CRUD with SHA-256 hashing
  - `SetState` / `GetStateValue` / `DeleteState` — OIDC state token storage (JSON-encoded `StateData`)

- **`setup/`** — Configuration singleton with environment variable loading
  - `GetConfig()` — Thread-safe singleton pattern with `sync.Once`
  - Loads `.env` file based on `GO_ENV` (ignores in production)

- **`utils/`** — HTTP client with custom root CA for internal TLS communication

### Key Implementation Details

- **Module name:** `bff` (see `go.mod`) — import internal packages as `bff/redis`, `bff/proxy`, etc.
- **Redis connection:** New client created per operation (no connection pooling)
- **Session hashing:** SHA-256 applied to session IDs before Redis storage
- **Configuration:** Environment variables with fallback defaults in `setup/config.go`
- **Logging:** JSON-structured logging via `log/slog` (level: Debug)
- **TLS:** Custom HTTP client loads root CA from `ROOT_CA_FILE` for Keycloak communication

## Development Commands

### Setup

```bash
# Copy environment template
cp .env.example .env

# Edit .env as needed (especially OAUTH2_CLIENT_SECRET if using Keycloak)
```

### Build & Run

```bash
# Build
task build
# or: go build ./...

# Lint (go vet)
task lint

# Run locally (requires Redis on redis:6379)
task run
# or: go run main.go
```

### Testing

```bash
# Run all tests (automatically starts/stops Redis via Docker Compose)
task test

# Run specific test
REDIS_ADDR=localhost:6379 go test ./proxy -run TestProxyHandler_PathRewriting

# Manual Redis for testing
docker compose -f compose.test.yml up -d
REDIS_ADDR=localhost:6379 go test ./...
docker compose -f compose.test.yml down
```

**Test environment:**
- Uses `compose.test.yml` to spin up Redis on port 6379
- `task test` automatically runs `test:up` (start Redis), executes tests with `REDIS_ADDR=localhost:6379`, then runs `test:down` (cleanup)
- Tests use `httptest.NewServer` for backend mocking (see `proxy/handler_test.go`)

### Docker

```bash
# Build image
docker build -t bff .

# Run container (requires network with redis/api services)
docker run --env-file .env -p 8080:8080 bff
```

## Environment Variables

Key variables (see `.env.example` for full list):

| Variable | Default | Description |
|----------|---------|-------------|
| `BFF_LISTEN_ADDR` | `:8080` | HTTP server bind address |
| `PROXY_TARGET` | `api:8081` | Backend API host:port |
| `OIDC_PROVIDER_URL` | `https://auth.local/idp/realms/go-proxy` | Keycloak realm URL |
| `OAUTH2_CLIENT_ID` | `bff` | Keycloak client ID |
| `OAUTH2_CLIENT_SECRET` | (empty) | Keycloak client secret |
| `OAUTH2_REDIRECT_URL` | `https://auth.local/api/auth/callback` | OAuth2 callback URL |
| `OAUTH2_TARGET_AUDIENCE` | `api` | Token exchange target audience (resource server client ID) |
| `REDIS_ADDR` | `redis:6379` | Redis connection string |
| `REDIS_PASSWORD` | (empty) | Redis password |
| `REDIS_ENCRYPTION_KEY` | **(required)** | AES-256-GCM key for session encryption — 64-char hex (generate: `openssl rand -hex 32`) |
| `SESSION_TTL` | `720h` (30 days) | Session expiration duration |
| `STATE_TTL` | `5m` | OAuth2 state token TTL (CSRF protection) |
| `SESSION_COOKIE_NAME` | `Session-Id` | Cookie name for session ID |
| `SESSION_COOKIE_SECURE` | `true` | Cookie Secure attribute |
| `SESSION_COOKIE_HTTPONLY` | `true` | Cookie HttpOnly attribute |
| `SESSION_COOKIE_SAMESITE` | `Strict` | Cookie SameSite attribute (`Strict`/`Lax`/`None`) |
| `ROOT_CA_FILE` | `./rootCA.crt` | Custom root CA for internal HTTPS |
| `ALLOWED_REDIRECT_PATH_PATTERN` | (empty) | Regex pattern for allowed post-login redirect paths |

**Testing override:**
- `REDIS_ADDR=localhost:6379` — Connect to local Redis (used in `task test`)

## Code Patterns

### Adding a New Endpoint

1. Create handler function in appropriate slice (or new directory)
2. Register in `main.go` using chi router:
   ```go
   r.Get("/your/path", yourpackage.YourHandler)
   // or for a subtree:
   r.Handle("/your/*", yourpackage.NewHandler(...))
   ```

### Accessing Configuration

```go
import "bff/setup"

cfg := setup.GetConfig()
listenAddr := cfg.BFFListenAddr
```

### Redis Operations

```go
import (
    "bff/redis"
    "github.com/google/uuid"
)

// Store session
sessionID := uuid.New().String()
redis.SetSession(sessionID, "user@example.com")

// Retrieve session
value, err := redis.GetSessionValue(sessionID)

// Delete session
redis.DeleteSession(sessionID)
```

### Testing with Mock Backend

```go
backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    // Assert request properties
    w.WriteHeader(http.StatusOK)
}))
defer backend.Close()

handler := proxy.NewHandler(backend.Listener.Addr().String())
req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
rr := httptest.NewRecorder()

handler.ServeHTTP(rr, req)
// Assert response
```

## Notes

- **No connection pooling:** Redis client is created/closed per operation (redis/redis.go)
- **Redis encryption:** Session data is encrypted with AES-256-GCM before storage. `REDIS_ENCRYPTION_KEY` (32-byte hex) is **required** at startup — missing key causes `log.Fatal`
- **Security TODOs:**
  - Consider implementing CSRF protection for non-OIDC state endpoints
