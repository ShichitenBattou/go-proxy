from uuid import UUID

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.domain.entities.user import User, UserRole
from app.infrastructure.models.user import UserModel


class SqlAlchemyUserRepository:
    def __init__(self, session: AsyncSession) -> None:
        self._session = session

    async def get_by_id(self, user_id: UUID) -> User | None:
        result = await self._session.execute(select(UserModel).where(UserModel.id == user_id))
        model = result.scalar_one_or_none()
        return self._to_entity(model) if model else None

    async def get_by_keycloak_sub(self, sub: str) -> User | None:
        result = await self._session.execute(select(UserModel).where(UserModel.keycloak_sub == sub))
        model = result.scalar_one_or_none()
        return self._to_entity(model) if model else None

    async def add(self, user: User) -> None:
        model = UserModel(
            id=user.id,
            keycloak_sub=user.keycloak_sub,
            role=user.role.value,
            is_active=user.is_active,
        )
        self._session.add(model)
        await self._session.flush()

    async def save(self, user: User) -> None:
        model = await self._session.get(UserModel, user.id)
        if model:
            model.role = user.role
            model.is_active = user.is_active
            await self._session.flush()

    def _to_entity(self, model: UserModel) -> User:
        return User(
            id=model.id,
            keycloak_sub=model.keycloak_sub,
            role=UserRole(model.role),
            is_active=model.is_active,
        )
