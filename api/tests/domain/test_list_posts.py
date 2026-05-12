from datetime import UTC, datetime
from unittest.mock import AsyncMock
from uuid import UUID, uuid4

import pytest

from app.application.queries.list_posts import ListPostsInteractor, ListPostsQuery
from app.domain.entities.post import Post


def make_post(author_id: UUID | None = None) -> Post:
    return Post(
        id=uuid4(),
        author_id=author_id or uuid4(),
        title="Title",
        body="Body",
        tags=[],
        created_at=datetime.now(UTC),
    )


@pytest.fixture
def post_repo() -> AsyncMock:
    return AsyncMock()


class TestListPostsInteractor:
    async def test_execute_list_all(self, post_repo: AsyncMock) -> None:
        posts = [make_post(), make_post()]
        post_repo.list_all.return_value = posts
        interactor = ListPostsInteractor(post_repo)

        result = await interactor.execute(ListPostsQuery(author_id=None))

        assert result == posts
        post_repo.list_all.assert_awaited_once()
        post_repo.list_by_author.assert_not_awaited()

    async def test_execute_list_by_author(self, post_repo: AsyncMock) -> None:
        author_id = uuid4()
        posts = [make_post(author_id=author_id)]
        post_repo.list_by_author.return_value = posts
        interactor = ListPostsInteractor(post_repo)

        result = await interactor.execute(ListPostsQuery(author_id=author_id))

        assert result == posts
        post_repo.list_by_author.assert_awaited_once_with(author_id)
        post_repo.list_all.assert_not_awaited()

    async def test_execute_passes_limit_and_offset(self, post_repo: AsyncMock) -> None:
        post_repo.list_all.return_value = []
        interactor = ListPostsInteractor(post_repo)

        await interactor.execute(ListPostsQuery(author_id=None, limit=5, offset=10))

        post_repo.list_all.assert_awaited_once_with(5, 10)
