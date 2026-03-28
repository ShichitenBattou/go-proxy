from unittest.mock import AsyncMock
from uuid import UUID, uuid4

import pytest

from app.application.commands.create_post import CreatePostCommand, CreatePostInteractor
from app.domain.entities.post import Post
from app.domain.entities.user import User, UserRole
from app.domain.exceptions import UserNotAuthorizedError, UserNotFoundError


def make_user(role: UserRole = UserRole.USER, is_active: bool = True) -> User:
    return User(id=uuid4(), keycloak_sub="sub-test", role=role, is_active=is_active)


def make_command(author_id: UUID | None = None) -> CreatePostCommand:
    return CreatePostCommand(
        author_id=author_id or uuid4(),
        title="Hello",
        body="World",
        tags=["tag1"],
    )


@pytest.fixture
def user_repo() -> AsyncMock:
    return AsyncMock()


@pytest.fixture
def post_repo() -> AsyncMock:
    return AsyncMock()


class TestCreatePostInteractor:
    async def test_execute_success(self, user_repo: AsyncMock, post_repo: AsyncMock) -> None:
        user = make_user(role=UserRole.USER, is_active=True)
        user_repo.get_by_id.return_value = user
        interactor = CreatePostInteractor(post_repo, user_repo)

        post = await interactor.execute(make_command(author_id=user.id))

        assert isinstance(post, Post)
        assert post.author_id == user.id
        assert post.title == "Hello"
        assert post.body == "World"
        assert post.tags == ["tag1"]

    async def test_execute_raises_user_not_found(
        self, user_repo: AsyncMock, post_repo: AsyncMock
    ) -> None:
        user_repo.get_by_id.return_value = None
        interactor = CreatePostInteractor(post_repo, user_repo)
        missing_id = uuid4()

        with pytest.raises(UserNotFoundError) as exc_info:
            await interactor.execute(make_command(author_id=missing_id))

        assert exc_info.value.user_id == missing_id

    async def test_execute_raises_unauthorized_for_admin(
        self, user_repo: AsyncMock, post_repo: AsyncMock
    ) -> None:
        admin = make_user(role=UserRole.ADMIN)
        user_repo.get_by_id.return_value = admin
        interactor = CreatePostInteractor(post_repo, user_repo)

        with pytest.raises(UserNotAuthorizedError, match="Administrators cannot create posts"):
            await interactor.execute(make_command(author_id=admin.id))

    async def test_execute_raises_unauthorized_for_inactive_user(
        self, user_repo: AsyncMock, post_repo: AsyncMock
    ) -> None:
        inactive = make_user(role=UserRole.USER, is_active=False)
        user_repo.get_by_id.return_value = inactive
        interactor = CreatePostInteractor(post_repo, user_repo)

        with pytest.raises(UserNotAuthorizedError, match="Inactive users cannot create posts"):
            await interactor.execute(make_command(author_id=inactive.id))

    async def test_execute_calls_post_repository_add(
        self, user_repo: AsyncMock, post_repo: AsyncMock
    ) -> None:
        user = make_user()
        user_repo.get_by_id.return_value = user
        interactor = CreatePostInteractor(post_repo, user_repo)

        await interactor.execute(make_command(author_id=user.id))

        post_repo.add.assert_awaited_once()
        added: Post = post_repo.add.call_args[0][0]
        assert added.author_id == user.id
