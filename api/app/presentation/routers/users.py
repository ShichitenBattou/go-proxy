from typing import Annotated
from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel

from app.application.commands.create_user import CreateUserCommand, CreateUserInteractor
from app.application.queries.get_user import GetUserInteractor
from app.domain.entities.user import UserRole
from app.presentation.dependencies import get_create_user_interactor, get_get_user_interactor

router = APIRouter(prefix="/users", tags=["users"])


class CreateUserRequest(BaseModel):
    keycloak_sub: str


class UserResponse(BaseModel):
    id: UUID
    keycloak_sub: str
    role: UserRole
    is_active: bool


@router.post("/", status_code=201, response_model=UserResponse)
async def create_user(
    body: CreateUserRequest,
    interactor: Annotated[CreateUserInteractor, Depends(get_create_user_interactor)],
) -> UserResponse:
    user = await interactor.execute(CreateUserCommand(keycloak_sub=body.keycloak_sub))
    return UserResponse(
        id=user.id,
        keycloak_sub=user.keycloak_sub,
        role=user.role,
        is_active=user.is_active,
    )


@router.get("/{user_id}", response_model=UserResponse)
async def get_user(
    user_id: UUID,
    interactor: Annotated[GetUserInteractor, Depends(get_get_user_interactor)],
) -> UserResponse:
    user = await interactor.execute(user_id)
    if not user:
        raise HTTPException(status_code=404, detail="User not found")
    return UserResponse(
        id=user.id,
        keycloak_sub=user.keycloak_sub,
        role=user.role,
        is_active=user.is_active,
    )
