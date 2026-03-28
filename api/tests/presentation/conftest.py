from collections.abc import AsyncGenerator
from uuid import UUID, uuid4

import pytest
from fastapi import FastAPI
from httpx import ASGITransport, AsyncClient

from app.application.commands.create_post import CreatePostInteractor
from app.application.commands.create_user import CreateUserInteractor
from app.application.queries.get_user import GetUserInteractor
from app.application.queries.list_posts import ListPostsInteractor
from app.domain.entities.post import Post
from app.domain.entities.user import User, UserRole
from app.presentation.dependencies import (
    get_create_post_interactor,
    get_create_user_interactor,
    get_current_user,
    get_get_user_interactor,
    get_list_posts_interactor,
    get_post_repo,
    get_user_repo,
)
from app.presentation.routers import posts, users

# ---------------------------------------------------------------------------
# In-memory repository implementations
# ---------------------------------------------------------------------------


class InMemoryUserRepository:
    def __init__(self) -> None:
        self._store: dict[UUID, User] = {}

    async def get_by_id(self, user_id: UUID) -> User | None:
        return self._store.get(user_id)

    async def get_by_keycloak_sub(self, sub: str) -> User | None:
        return next((u for u in self._store.values() if u.keycloak_sub == sub), None)

    async def add(self, user: User) -> None:
        self._store[user.id] = user

    async def save(self, user: User) -> None:
        self._store[user.id] = user


class InMemoryPostRepository:
    def __init__(self) -> None:
        self._store: dict[UUID, Post] = {}

    async def get_by_id(self, post_id: UUID) -> Post | None:
        return self._store.get(post_id)

    async def list_by_author(self, author_id: UUID) -> list[Post]:
        return [p for p in self._store.values() if p.author_id == author_id]

    async def list_all(self, limit: int = 20, offset: int = 0) -> list[Post]:
        all_posts = list(self._store.values())
        return all_posts[offset : offset + limit]

    async def add(self, post: Post) -> None:
        self._store[post.id] = post

    async def save(self, post: Post) -> None:
        self._store[post.id] = post


# ---------------------------------------------------------------------------
# Repository fixtures — function-scoped so each test gets a clean slate
# ---------------------------------------------------------------------------


@pytest.fixture
def user_repo() -> InMemoryUserRepository:
    return InMemoryUserRepository()


@pytest.fixture
def post_repo() -> InMemoryPostRepository:
    return InMemoryPostRepository()


# ---------------------------------------------------------------------------
# Pre-populated user fixtures
# ---------------------------------------------------------------------------


@pytest.fixture
def test_user(user_repo: InMemoryUserRepository) -> User:
    user = User(id=uuid4(), keycloak_sub="sub-test-user", role=UserRole.USER, is_active=True)
    user_repo._store[user.id] = user
    return user


@pytest.fixture
def admin_user(user_repo: InMemoryUserRepository) -> User:
    user = User(id=uuid4(), keycloak_sub="sub-admin", role=UserRole.ADMIN, is_active=True)
    user_repo._store[user.id] = user
    return user


@pytest.fixture
def inactive_user(user_repo: InMemoryUserRepository) -> User:
    user = User(id=uuid4(), keycloak_sub="sub-inactive", role=UserRole.USER, is_active=False)
    user_repo._store[user.id] = user
    return user


# ---------------------------------------------------------------------------
# FastAPI test application — avoids main.py lifespan (Keycloak / JWKS)
# ---------------------------------------------------------------------------


def _make_test_app() -> FastAPI:
    app = FastAPI()
    app.include_router(users.router)
    app.include_router(posts.router)
    return app


# ---------------------------------------------------------------------------
# AsyncClient fixture — wires in-memory repos via dependency_overrides
# ---------------------------------------------------------------------------


@pytest.fixture
async def client(
    user_repo: InMemoryUserRepository,
    post_repo: InMemoryPostRepository,
    test_user: User,
) -> AsyncGenerator[AsyncClient, None]:
    app = _make_test_app()

    app.dependency_overrides[get_user_repo] = lambda: user_repo
    app.dependency_overrides[get_post_repo] = lambda: post_repo
    app.dependency_overrides[get_current_user] = lambda: test_user

    # Interactors must also be overridden because their default Depends chain
    # goes through get_user_repo / get_post_repo which expect a DB session.
    app.dependency_overrides[get_create_user_interactor] = lambda: CreateUserInteractor(user_repo)
    app.dependency_overrides[get_get_user_interactor] = lambda: GetUserInteractor(user_repo)
    app.dependency_overrides[get_create_post_interactor] = lambda: CreatePostInteractor(
        post_repo, user_repo
    )
    app.dependency_overrides[get_list_posts_interactor] = lambda: ListPostsInteractor(post_repo)

    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
        yield ac

    app.dependency_overrides.clear()
