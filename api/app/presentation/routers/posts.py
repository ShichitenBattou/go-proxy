from datetime import datetime
from typing import Annotated
from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel

from app.application.commands.create_post import CreatePostCommand, CreatePostInteractor
from app.application.exceptions import UserNotAuthorizedError
from app.application.queries.get_post import GetPostInteractor, GetPostQuery
from app.application.queries.list_posts import ListPostsInteractor, ListPostsQuery
from app.domain.exceptions import PostNotFoundError, UserNotFoundError
from app.presentation.dependencies import (
    CurrentUser,
    get_create_post_interactor,
    get_get_post_interactor,
    get_list_posts_interactor,
)

router = APIRouter(prefix="/posts", tags=["posts"])


class CreatePostRequest(BaseModel):
    title: str
    body: str
    tags: list[str] = []


class PostResponse(BaseModel):
    id: UUID
    author_id: UUID
    title: str
    body: str
    tags: list[str]
    created_at: datetime
    version: int


@router.post(
    "",
    status_code=201,
    response_model=PostResponse,
    summary="Create a post",
    description=(
        "新しい投稿を作成する。\n\n"
        "**投稿可能条件**: `role=USER` かつ `is_active=True` のユーザーのみ。"
        "管理者（`role=ADMIN`）および無効化ユーザーは投稿不可（**403**）。\n\n"
        "投稿者が存在しない場合は **404** を返す。"
    ),
)
async def create_post(
    body: CreatePostRequest,
    current_user: CurrentUser,
    interactor: Annotated[CreatePostInteractor, Depends(get_create_post_interactor)],
) -> PostResponse:
    try:
        post = await interactor.execute(
            CreatePostCommand(
                author_id=current_user.id,
                title=body.title,
                body=body.body,
                tags=body.tags,
            )
        )
    except UserNotFoundError as e:
        raise HTTPException(status_code=404, detail=str(e))
    except UserNotAuthorizedError as e:
        raise HTTPException(status_code=403, detail=str(e))

    return PostResponse(
        id=post.id,
        author_id=post.author_id,
        title=post.title,
        body=post.body,
        tags=post.tags,
        created_at=post.created_at,
        version=post.version,
    )


@router.get(
    "",
    response_model=list[PostResponse],
    summary="List posts",
    description=(
        "投稿一覧を返す。\n\n"
        "| クエリパラメータ | 型 | 説明 |\n"
        "|---|---|---|\n"
        "| `author_id` | UUID (optional) | 指定した著者の投稿のみ絞り込む |\n"
        "| `limit` | int (default: 20) | 取得件数の上限 |\n"
        "| `offset` | int (default: 0) | 取得開始位置（ページネーション用） |"
    ),
)
async def list_posts(
    current_user: CurrentUser,
    interactor: Annotated[ListPostsInteractor, Depends(get_list_posts_interactor)],
    author_id: UUID | None = None,
    limit: int = 20,
    offset: int = 0,
) -> list[PostResponse]:
    posts = await interactor.execute(
        ListPostsQuery(author_id=author_id, limit=limit, offset=offset)
    )
    return [
        PostResponse(
            id=p.id,
            author_id=p.author_id,
            title=p.title,
            body=p.body,
            tags=p.tags,
            created_at=p.created_at,
            version=p.version,
        )
        for p in posts
    ]


@router.get(
    "/{post_id}",
    response_model=PostResponse,
    summary="Get post by ID",
    description="指定した `post_id` の投稿を返す。存在しない場合は **404** を返す。",
)
async def get_post(
    post_id: UUID,
    current_user: CurrentUser,
    interactor: Annotated[GetPostInteractor, Depends(get_get_post_interactor)],
) -> PostResponse:
    try:
        post = await interactor.execute(GetPostQuery(post_id=post_id))
    except PostNotFoundError as e:
        raise HTTPException(status_code=404, detail=str(e))

    return PostResponse(
        id=post.id,
        author_id=post.author_id,
        title=post.title,
        body=post.body,
        tags=post.tags,
        created_at=post.created_at,
        version=post.version,
    )
