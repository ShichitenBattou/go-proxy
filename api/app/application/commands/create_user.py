from dataclasses import dataclass
from uuid import uuid4

from app.domain.entities.user import User
from app.domain.ports.user_repository import UserRepository


@dataclass
class CreateUserCommand:
    keycloak_sub: str


class CreateUserInteractor:
    def __init__(self, user_repository: UserRepository) -> None:
        self._user_repository = user_repository

    async def execute(self, command: CreateUserCommand) -> tuple[User, bool]:
        existing = await self._user_repository.get_by_keycloak_sub(command.keycloak_sub)
        if existing:
            return existing, False

        user = User(id=uuid4(), keycloak_sub=command.keycloak_sub)
        await self._user_repository.add(user)
        return user, True
