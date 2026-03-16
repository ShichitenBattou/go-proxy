from uuid import UUID

from app.domain.entities.user import User
from app.domain.ports.user_repository import UserRepository


class GetUserInteractor:
    def __init__(self, user_repository: UserRepository) -> None:
        self._user_repository = user_repository

    async def execute(self, user_id: UUID) -> User | None:
        return await self._user_repository.get_by_id(user_id)
