from typing import Annotated
from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException, Response
from pydantic import BaseModel

from app.application.commands.create_user import CreateUserCommand, CreateUserInteractor
from app.application.commands.deactivate_user import DeactivateUserCommand, DeactivateUserInteractor
from app.application.queries.get_user import GetUserInteractor
from app.domain.entities.user import UserRole
from app.domain.exceptions import UserNotFoundError
from app.presentation.auth import CurrentSub
from app.presentation.dependencies import (
    CurrentUser,
    get_create_user_interactor,
    get_deactivate_user_interactor,
    get_get_user_interactor,
)

router = APIRouter(prefix="/users", tags=["users"])


class UserResponse(BaseModel):
    id: UUID
    keycloak_sub: str
    role: UserRole
    is_active: bool


@router.post(
    "",
    status_code=201,
    response_model=UserResponse,
    summary="Upsert user by Keycloak sub",
    description=(
        "Keycloak の `sub` クレームをキーにユーザーを登録する。\n\n"
        "- 同一 `keycloak_sub` が既存の場合は **200** を返す（冪等）\n"
        "- 新規作成の場合は **201** を返す"
    ),
)
async def create_user(
    response: Response,
    keycloak_sub: CurrentSub,
    interactor: Annotated[CreateUserInteractor, Depends(get_create_user_interactor)],
) -> UserResponse:
    user, created = await interactor.execute(CreateUserCommand(keycloak_sub=keycloak_sub))
    if not created:
        response.status_code = 200
    return UserResponse(
        id=user.id,
        keycloak_sub=user.keycloak_sub,
        role=user.role,
        is_active=user.is_active,
    )


@router.get(
    "/{user_id}",
    response_model=UserResponse,
    summary="Get user by ID",
    description="指定した `user_id` のユーザーを返す。存在しない場合は **404** を返す。",
)
async def get_user(
    user_id: UUID,
    current_user: CurrentUser,
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


@router.delete(
    "/{user_id}",
    status_code=204,
    summary="Deactivate user (soft delete)",
    description=(
        "ユーザーを**論理削除**（無効化）する。物理削除ではなくレコードの `is_active` を `False` に"
        "セットするため、データは保持される。\n\n"
        "存在しないユーザーを指定した場合は **404** を返す。"
    ),
)
async def deactivate_user(
    user_id: UUID,
    current_user: CurrentUser,
    interactor: Annotated[DeactivateUserInteractor, Depends(get_deactivate_user_interactor)],
) -> None:
    try:
        await interactor.execute(DeactivateUserCommand(user_id=user_id))
    except UserNotFoundError:
        raise HTTPException(status_code=404, detail="User not found")
