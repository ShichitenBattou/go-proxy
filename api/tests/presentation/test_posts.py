from datetime import UTC, datetime
from uuid import uuid4

from fastapi import FastAPI
from httpx import ASGITransport, AsyncClient

from app.application.commands.create_post import CreatePostInteractor
from app.application.queries.list_posts import ListPostsInteractor
from app.domain.entities.post import Post
from app.domain.entities.user import User
from app.presentation.dependencies import (
    get_create_post_interactor,
    get_current_user,
    get_list_posts_interactor,
    get_post_repo,
    get_user_repo,
)
from app.presentation.routers import posts, users
from tests.presentation.conftest import InMemoryPostRepository, InMemoryUserRepository

_CREATE_PAYLOAD = {"title": "My Post", "body": "Content here", "tags": ["python", "fastapi"]}


class TestCreatePost:
    async def test_create_post_returns_201(self, client: AsyncClient) -> None:
        response = await client.post("/posts", json=_CREATE_PAYLOAD)

        assert response.status_code == 201

    async def test_create_post_response_schema(self, client: AsyncClient) -> None:
        response = await client.post("/posts", json=_CREATE_PAYLOAD)

        assert response.status_code == 201
        body = response.json()
        for key in ("id", "author_id", "title", "body", "tags", "created_at", "version"):
            assert key in body, f"Missing key: {key}"
        assert body["title"] == "My Post"
        assert body["body"] == "Content here"
        assert body["tags"] == ["python", "fastapi"]
        assert body["version"] == 1

    async def test_create_post_admin_returns_403(
        self,
        user_repo: InMemoryUserRepository,
        post_repo: InMemoryPostRepository,
        admin_user: User,
    ) -> None:
        app = FastAPI()
        app.include_router(users.router)
        app.include_router(posts.router)

        app.dependency_overrides[get_user_repo] = lambda: user_repo
        app.dependency_overrides[get_post_repo] = lambda: post_repo
        app.dependency_overrides[get_current_user] = lambda: admin_user
        app.dependency_overrides[get_create_post_interactor] = lambda: CreatePostInteractor(
            post_repo, user_repo
        )
        app.dependency_overrides[get_list_posts_interactor] = lambda: ListPostsInteractor(post_repo)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
            response = await ac.post("/posts", json=_CREATE_PAYLOAD)

        assert response.status_code == 403

    async def test_create_post_inactive_user_returns_403(
        self,
        user_repo: InMemoryUserRepository,
        post_repo: InMemoryPostRepository,
        inactive_user: User,
    ) -> None:
        app = FastAPI()
        app.include_router(users.router)
        app.include_router(posts.router)

        app.dependency_overrides[get_user_repo] = lambda: user_repo
        app.dependency_overrides[get_post_repo] = lambda: post_repo
        app.dependency_overrides[get_current_user] = lambda: inactive_user
        app.dependency_overrides[get_create_post_interactor] = lambda: CreatePostInteractor(
            post_repo, user_repo
        )
        app.dependency_overrides[get_list_posts_interactor] = lambda: ListPostsInteractor(post_repo)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
            response = await ac.post("/posts", json=_CREATE_PAYLOAD)

        assert response.status_code == 403


class TestListPosts:
    async def test_list_posts_returns_200(self, client: AsyncClient) -> None:
        response = await client.get("/posts")

        assert response.status_code == 200

    async def test_list_posts_empty(self, client: AsyncClient) -> None:
        response = await client.get("/posts")

        assert response.status_code == 200
        assert response.json() == []

    async def test_list_posts_by_author(
        self,
        client: AsyncClient,
        post_repo: InMemoryPostRepository,
        test_user: User,
    ) -> None:
        other_author_id = uuid4()
        now = datetime.now(UTC)

        # Add a post for test_user and one for another author.
        await post_repo.add(
            Post(
                id=uuid4(),
                author_id=test_user.id,
                title="Mine",
                body="body",
                tags=[],
                created_at=now,
            )
        )
        await post_repo.add(
            Post(
                id=uuid4(),
                author_id=other_author_id,
                title="Not mine",
                body="body",
                tags=[],
                created_at=now,
            )
        )

        response = await client.get(f"/posts?author_id={test_user.id}")

        assert response.status_code == 200
        body = response.json()
        assert len(body) == 1
        assert body[0]["title"] == "Mine"
        assert body[0]["author_id"] == str(test_user.id)

    async def test_list_posts_requires_auth(self, unauthenticated_client: AsyncClient) -> None:
        response = await unauthenticated_client.get("/posts")

        assert response.status_code == 401


class TestGetPost:
    async def test_get_post_returns_200(
        self,
        client: AsyncClient,
        post_repo: InMemoryPostRepository,
        test_user: User,
    ) -> None:
        now = datetime.now(UTC)
        post = Post(
            id=uuid4(),
            author_id=test_user.id,
            title="Detail Post",
            body="Detail body",
            tags=["tag1"],
            created_at=now,
        )
        await post_repo.add(post)

        response = await client.get(f"/posts/{post.id}")

        assert response.status_code == 200
        body = response.json()
        assert body["id"] == str(post.id)
        assert body["author_id"] == str(test_user.id)
        assert body["title"] == "Detail Post"
        assert body["body"] == "Detail body"
        assert body["tags"] == ["tag1"]
        assert body["version"] == 1

    async def test_get_post_not_found_returns_404(self, client: AsyncClient) -> None:
        missing_id = str(uuid4())

        response = await client.get(f"/posts/{missing_id}")

        assert response.status_code == 404

    async def test_get_post_requires_auth(self, unauthenticated_client: AsyncClient) -> None:
        some_id = str(uuid4())

        response = await unauthenticated_client.get(f"/posts/{some_id}")

        assert response.status_code == 401


class TestInactiveUserRestriction:
    async def test_create_post_inactive_returns_403(
        self, inactive_client: AsyncClient
    ) -> None:
        response = await inactive_client.post(
            "/posts", json={"title": "t", "body": "b", "tags": []}
        )

        assert response.status_code == 403

    async def test_list_posts_inactive_returns_403(
        self, inactive_client: AsyncClient
    ) -> None:
        response = await inactive_client.get("/posts")

        assert response.status_code == 403

    async def test_get_post_inactive_returns_403(
        self, inactive_client: AsyncClient
    ) -> None:
        response = await inactive_client.get(f"/posts/{uuid4()}")

        assert response.status_code == 403
