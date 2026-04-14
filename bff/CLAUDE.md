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
3. **Session Rotation** — Generates new session ID on every response to prevent fixation attacks
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
3. Forward to backend with `Request-Id` header
4. Delete old session from Redis
5. Generate new session ID and set `Set-Cookie` header
6. Store new session in Redis with TTL

### Vertical Slice Architecture

The codebase follows **Vertical Slice Architecture** — features are organized by use case, not by technical layer:

- **`auth/`** — OIDC login flow (`/auth/login`, `/auth/callback`)
  - `LoginHandler` — Creates state token, stores in Redis, redirects to Keycloak
  - `CallbackHandler` — Exchanges authorization code for tokens (incomplete implementation)

- **`proxy/`** — Reverse proxy with session validation (`/api/*`)
  - `NewHandler` — Returns `http.Handler` that proxies to backend
  - Path rewriting: removes `/api` prefix
  - Session rotation: validates incoming session, creates new session on response

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
| `OAUTH2_CLIENT_ID` | `api` | Keycloak client ID |
| `OAUTH2_CLIENT_SECRET` | (empty) | Keycloak client secret |
| `OAUTH2_REDIRECT_URL` | `https://auth.local/api/auth/callback` | OAuth2 callback URL |
| `REDIS_ADDR` | `redis:6379` | Redis connection string |
| `SESSION_TTL` | `720h` (30 days) | Session expiration duration |
| `SESSION_COOKIE_NAME` | `Session-Id` | Cookie name for session ID |
| `ROOT_CA_FILE` | `./rootCA.crt` | Custom root CA for internal HTTPS |

**Testing override:**
- `REDIS_ADDR=localhost:6379` — Connect to local Redis (used in `task test`)

## Code Patterns

### Adding a New Endpoint

1. Create handler function in appropriate slice (or new directory)
2. Register in `main.go`:
   ```go
   http.HandleFunc("/your/path", yourpackage.YourHandler)
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

- **Incomplete implementation:** `auth/CallbackHandler` (auth/handler.go:69) exchanges code for token but doesn't create session yet — just returns HTTP 501
- **Session rotation:** Every API response triggers session deletion + creation, which may cause race conditions under high concurrency
- **No connection pooling:** Redis client is created/closed per operation (redis/redis.go:25)
- **Security TODOs:**
  - Add `HttpOnly` and `SameSite` attributes to session cookie (setup/config.go:34)
  - Consider implementing CSRF protection
  - Validate state token expiration in callback handler
