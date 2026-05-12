from datetime import datetime
from typing import Annotated
from uuid import UUID

from fastapi import APIRouter, Depends, Query
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


@router.get(
    "/posts",
    response_model=list[PostResponse],
    summary="List posts (public)",
    description="認証不要で投稿一覧を返す公開エンドポイント。`author_id`・`limit`・`offset` でフィルタ・ページネーション可能。",
)
async def list_public_posts(
    interactor: Annotated[ListPostsInteractor, Depends(get_list_posts_interactor)],
    author_id: UUID | None = Query(default=None, description="指定した著者の投稿のみ絞り込む"),
    limit: int = Query(default=20, description="取得件数の上限"),
    offset: int = Query(default=0, description="取得開始位置（ページネーション用）"),
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
