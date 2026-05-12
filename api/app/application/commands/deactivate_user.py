from dataclasses import dataclass
from uuid import UUID

from app.domain.exceptions import UserNotFoundError
from app.domain.ports.user_repository import UserRepository


@dataclass
class DeactivateUserCommand:
    user_id: UUID


class DeactivateUserInteractor:
    def __init__(self, user_repository: UserRepository) -> None:
        self._user_repository = user_repository

    async def execute(self, command: DeactivateUserCommand) -> None:
        user = await self._user_repository.get_by_id(command.user_id)
        if not user:
            raise UserNotFoundError(command.user_id)
        user.is_active = False
        await self._user_repository.save(user)
