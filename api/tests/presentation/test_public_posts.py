from datetime import UTC, datetime
from uuid import uuid4

from httpx import AsyncClient

from app.domain.entities.post import Post
from app.domain.entities.user import User
from tests.presentation.conftest import InMemoryPostRepository


class TestListPublicPosts:
    async def test_list_public_posts_returns_200_without_auth(
        self, unauthenticated_client: AsyncClient
    ) -> None:
        # Public endpoint must be accessible without a Bearer token.
        response = await unauthenticated_client.get("/public/posts")

        assert response.status_code == 200

    async def test_list_public_posts_empty(self, unauthenticated_client: AsyncClient) -> None:
        response = await unauthenticated_client.get("/public/posts")

        assert response.status_code == 200
        assert response.json() == []

    async def test_list_public_posts_returns_posts(
        self,
        unauthenticated_client: AsyncClient,
        post_repo: InMemoryPostRepository,
        test_user: User,
    ) -> None:
        now = datetime.now(UTC)
        post = Post(
            id=uuid4(),
            author_id=test_user.id,
            title="Public Post",
            body="Visible to all",
            tags=["open"],
            created_at=now,
        )
        await post_repo.add(post)

        response = await unauthenticated_client.get("/public/posts")

        assert response.status_code == 200
        body = response.json()
        assert len(body) == 1
        assert body[0]["id"] == str(post.id)
        assert body[0]["title"] == "Public Post"
        assert body[0]["tags"] == ["open"]

    async def test_list_public_posts_by_author(
        self,
        unauthenticated_client: AsyncClient,
        post_repo: InMemoryPostRepository,
        test_user: User,
    ) -> None:
        other_author_id = uuid4()
        now = datetime.now(UTC)

        await post_repo.add(
            Post(
                id=uuid4(),
                author_id=test_user.id,
                title="My Public Post",
                body="body",
                tags=[],
                created_at=now,
            )
        )
        await post_repo.add(
            Post(
                id=uuid4(),
                author_id=other_author_id,
                title="Other's Post",
                body="body",
                tags=[],
                created_at=now,
            )
        )

        response = await unauthenticated_client.get(f"/public/posts?author_id={test_user.id}")

        assert response.status_code == 200
        body = response.json()
        assert len(body) == 1
        assert body[0]["title"] == "My Public Post"
        assert body[0]["author_id"] == str(test_user.id)
