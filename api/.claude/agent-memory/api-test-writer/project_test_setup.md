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
    ├── conftest.py         # in-memory repos + AsyncClient fixture
    ├── test_posts.py
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
- `client(user_repo, post_repo, test_user)` → `httpx.AsyncClient` with all dependency_overrides wired

## dependency_overrides pattern

A **test-only FastAPI app** (no lifespan) is created in conftest to avoid Keycloak/JWKS initialization:

```python
app = FastAPI()
app.include_router(users.router)
app.include_router(posts.router)
```

Overridden dependencies:
- `get_user_repo` → `lambda: user_repo`
- `get_post_repo` → `lambda: post_repo`
- `get_current_user` → `lambda: test_user`  (or admin/inactive variant)
- `get_create_user_interactor` → `lambda: CreateUserInteractor(user_repo)`
- `get_get_user_interactor` → `lambda: GetUserInteractor(user_repo)`
- `get_create_post_interactor` → `lambda: CreatePostInteractor(post_repo, user_repo)`
- `get_list_posts_interactor` → `lambda: ListPostsInteractor(post_repo)`

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
