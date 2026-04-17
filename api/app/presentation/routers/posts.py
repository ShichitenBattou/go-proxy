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


@router.post("", status_code=201, response_model=PostResponse)
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


@router.get("", response_model=list[PostResponse])
async def list_posts(
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


@router.get("/{post_id}", response_model=PostResponse)
async def get_post(
    post_id: UUID,
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
