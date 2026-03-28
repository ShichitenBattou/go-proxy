from unittest.mock import AsyncMock
from uuid import uuid4

import pytest

from app.application.commands.create_user import CreateUserCommand, CreateUserInteractor
from app.domain.entities.user import User, UserRole


def make_existing_user(sub: str) -> User:
    return User(id=uuid4(), keycloak_sub=sub, role=UserRole.USER, is_active=True)


@pytest.fixture
def user_repo() -> AsyncMock:
    return AsyncMock()


class TestCreateUserInteractor:
    async def test_execute_creates_new_user(self, user_repo: AsyncMock) -> None:
        user_repo.get_by_keycloak_sub.return_value = None
        interactor = CreateUserInteractor(user_repo)

        result = await interactor.execute(CreateUserCommand(keycloak_sub="new-sub"))

        assert isinstance(result, User)
        assert result.keycloak_sub == "new-sub"
        assert result.role == UserRole.USER
        assert result.is_active is True

    async def test_execute_idempotent(self, user_repo: AsyncMock) -> None:
        existing = make_existing_user("existing-sub")
        user_repo.get_by_keycloak_sub.return_value = existing
        interactor = CreateUserInteractor(user_repo)

        result = await interactor.execute(CreateUserCommand(keycloak_sub="existing-sub"))

        assert result is existing
        user_repo.add.assert_not_awaited()

    async def test_execute_calls_user_repository_add_for_new_user(
        self, user_repo: AsyncMock
    ) -> None:
        user_repo.get_by_keycloak_sub.return_value = None
        interactor = CreateUserInteractor(user_repo)

        await interactor.execute(CreateUserCommand(keycloak_sub="brand-new"))

        user_repo.add.assert_awaited_once()
        added: User = user_repo.add.call_args[0][0]
        assert added.keycloak_sub == "brand-new"
