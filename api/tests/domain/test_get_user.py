from unittest.mock import AsyncMock
from uuid import uuid4

import pytest

from app.application.queries.get_user import GetUserInteractor
from app.domain.entities.user import User, UserRole


@pytest.fixture
def user_repo() -> AsyncMock:
    return AsyncMock()


class TestGetUserInteractor:
    async def test_execute_returns_user(self, user_repo: AsyncMock) -> None:
        user_id = uuid4()
        expected = User(id=user_id, keycloak_sub="some-sub", role=UserRole.USER, is_active=True)
        user_repo.get_by_id.return_value = expected
        interactor = GetUserInteractor(user_repo)

        result = await interactor.execute(user_id)

        assert result is expected
        user_repo.get_by_id.assert_awaited_once_with(user_id)

    async def test_execute_returns_none(self, user_repo: AsyncMock) -> None:
        user_repo.get_by_id.return_value = None
        interactor = GetUserInteractor(user_repo)

        result = await interactor.execute(uuid4())

        assert result is None
