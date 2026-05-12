---
name: project_test_setup
description: Test infrastructure conventions: fixture layout, in-memory repos, dependency_overrides pattern for HTTP tests
type: project
---

## Test directory layout

```
api/tests/
├── __init__.py
├── domain/
│   ├── __init__.py
│   ├── test_create_post.py
│   ├── test_create_user.py
│   ├── test_get_user.py
│   └── test_list_posts.py
└── presentation/
    ├── __init__.py
    ├── conftest.py              # in-memory repos + AsyncClient fixtures
    ├── test_auth.py
    ├── test_posts.py
    ├── test_public_posts.py     # GET /public/posts (no auth)
    └── test_users.py
```

## asyncio_mode

`asyncio_mode = "auto"` is set in `pyproject.toml`. No `@pytest.mark.asyncio` decorator needed.

## Domain/application unit tests

- Use `unittest.mock.AsyncMock` for all repository mocks (no pytest-mock).
- Group tests in a class per interactor (e.g. `class TestCreatePostInteractor:`).
- Domain entities: `User`, `Post` — both plain dataclasses, easy to instantiate directly.

## In-memory repositories (presentation/conftest.py)

`InMemoryUserRepository` and `InMemoryPostRepository` implement the domain `Protocol` interfaces using a simple `dict[UUID, Entity]` store. Each fixture is function-scoped so tests are isolated.

Key fixtures defined in `tests/presentation/conftest.py`:
- `user_repo` → fresh `InMemoryUserRepository`
- `post_repo` → fresh `InMemoryPostRepository`
- `test_user(user_repo)` → `User(role=USER, is_active=True)` pre-inserted in user_repo
- `admin_user(user_repo)` → `User(role=ADMIN, is_active=True)` pre-inserted
- `inactive_user(user_repo)` → `User(role=USER, is_active=False)` pre-inserted
- `client(user_repo, post_repo, test_user)` → `httpx.AsyncClient` with all dependency_overrides wired (authenticated as test_user)
- `unauthenticated_client(user_repo, post_repo)` → `httpx.AsyncClient` where `get_current_sub` is overridden to raise 401; used to verify auth-required endpoints reject unauthenticated requests, and to call public endpoints without auth

## dependency_overrides pattern

A **test-only FastAPI app** (no lifespan) is created in conftest to avoid Keycloak/JWKS initialization:

```python
app = FastAPI()
app.include_router(users.router)
app.include_router(posts.router)
app.include_router(public.router)   # prefix="/public"
```

Overridden dependencies (both `client` and `unauthenticated_client`):
- `get_user_repo` → `lambda: user_repo`
- `get_post_repo` → `lambda: post_repo`
- `get_create_user_interactor` → `lambda: CreateUserInteractor(user_repo)`
- `get_get_user_interactor` → `lambda: GetUserInteractor(user_repo)`
- `get_deactivate_user_interactor` → `lambda: DeactivateUserInteractor(user_repo)`
- `get_create_post_interactor` → `lambda: CreatePostInteractor(post_repo, user_repo)`
- `get_get_post_interactor` → `lambda: GetPostInteractor(post_repo)`
- `get_list_posts_interactor` → `lambda: ListPostsInteractor(post_repo)`

Additional for `client` only:
- `get_current_user` → `lambda: test_user`

Additional for `unauthenticated_client` only:
- `get_current_sub` → raises `HTTPException(401)` — simulates missing Bearer token at the JWT layer

Note: Interactor-level overrides are necessary because their default Depends chain expects a DB session.

For tests that need a **different current_user** (admin, inactive), create a fresh `FastAPI()` + `AsyncClient` inline within the test rather than relying on the shared `client` fixture.

## Transport

```python
from httpx import ASGITransport, AsyncClient
async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
    ...
```

## Dev dependencies added

`pyproject.toml [dependency-groups] dev`:
- `pytest>=8.0`
- `pytest-asyncio>=0.24`
- `httpx>=0.27`

**Why:** These are the minimum needed for async HTTP integration tests without hitting the real database or Keycloak.
**How to apply:** Run `uv sync` after editing pyproject.toml to install.
