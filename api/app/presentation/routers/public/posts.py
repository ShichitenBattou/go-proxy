from datetime import datetime
from typing import Annotated
from uuid import UUID

from fastapi import APIRouter, Depends
from pydantic import BaseModel

from app.application.queries.list_posts import ListPostsInteractor, ListPostsQuery
from app.presentation.dependencies import get_list_posts_interactor

router = APIRouter()


class PostResponse(BaseModel):
    id: UUID
    author_id: UUID
    title: str
    body: str
    tags: list[str]
    created_at: datetime
    version: int


@router.get("/posts", response_model=list[PostResponse])
async def list_public_posts(
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
