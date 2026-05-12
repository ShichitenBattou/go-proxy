from typing import Annotated

from fastapi import Depends, HTTPException, status
from sqlalchemy.ext.asyncio import AsyncSession

from app.application.commands.create_post import CreatePostInteractor
from app.application.commands.create_user import CreateUserInteractor
from app.application.commands.deactivate_user import DeactivateUserInteractor
from app.application.queries.get_post import GetPostInteractor
from app.application.queries.get_user import GetUserInteractor
from app.application.queries.list_posts import ListPostsInteractor
from app.domain.entities.user import User
from app.infrastructure.database import get_db
from app.infrastructure.repositories.post_repository import SqlAlchemyPostRepository
from app.infrastructure.repositories.user_repository import SqlAlchemyUserRepository
from app.presentation.auth import CurrentSub

DbSession = Annotated[AsyncSession, Depends(get_db)]


def get_user_repo(session: DbSession) -> SqlAlchemyUserRepository:
    return SqlAlchemyUserRepository(session)


def get_post_repo(session: DbSession) -> SqlAlchemyPostRepository:
    return SqlAlchemyPostRepository(session)


UserRepoDep = Annotated[SqlAlchemyUserRepository, Depends(get_user_repo)]
PostRepoDep = Annotated[SqlAlchemyPostRepository, Depends(get_post_repo)]


def get_create_user_interactor(user_repo: UserRepoDep) -> CreateUserInteractor:
    return CreateUserInteractor(user_repo)


def get_deactivate_user_interactor(user_repo: UserRepoDep) -> DeactivateUserInteractor:
    return DeactivateUserInteractor(user_repo)


def get_create_post_interactor(
    post_repo: PostRepoDep,
    user_repo: UserRepoDep,
) -> CreatePostInteractor:
    return CreatePostInteractor(post_repo, user_repo)


def get_get_user_interactor(user_repo: UserRepoDep) -> GetUserInteractor:
    return GetUserInteractor(user_repo)


def get_get_post_interactor(post_repo: PostRepoDep) -> GetPostInteractor:
    return GetPostInteractor(post_repo)


def get_list_posts_interactor(post_repo: PostRepoDep) -> ListPostsInteractor:
    return ListPostsInteractor(post_repo)


async def get_current_user(
    keycloak_sub: CurrentSub,
    user_repo: UserRepoDep,
) -> User:
    """JWT の sub クレームから現在のユーザーを取得

    Args:
        keycloak_sub: JWT の sub クレーム（Keycloak のユーザー ID）
        user_repo: ユーザーリポジトリ

    Returns:
        認証済みユーザー

    Raises:
        HTTPException: ユーザーが DB に登録されていない場合（401）
        HTTPException: ユーザーが非アクティブの場合（403）
    """
    user = await user_repo.get_by_keycloak_sub(keycloak_sub)
    if not user:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="User not registered. Please register at POST /users/ first.",
            headers={"WWW-Authenticate": "Bearer"},
        )
    if not user.is_active:
        raise HTTPException(
            status_code=status.HTTP_403_FORBIDDEN,
            detail="Inactive users cannot perform resource operations.",
        )
    return user


CurrentUser = Annotated[User, Depends(get_current_user)]
