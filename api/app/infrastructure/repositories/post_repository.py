from uuid import UUID

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.domain.entities.post import Post
from app.infrastructure.models.post import PostModel


class SqlAlchemyPostRepository:
    def __init__(self, session: AsyncSession) -> None:
        self._session = session

    async def get_by_id(self, post_id: UUID) -> Post | None:
        model = await self._session.get(PostModel, post_id)
        return self._to_entity(model) if model else None

    async def list_by_author(self, author_id: UUID) -> list[Post]:
        result = await self._session.execute(
            select(PostModel)
            .where(PostModel.author_id == author_id)
            .order_by(PostModel.created_at.desc())
        )
        return [self._to_entity(m) for m in result.scalars()]

    async def list_all(self, limit: int = 20, offset: int = 0) -> list[Post]:
        result = await self._session.execute(
            select(PostModel)
            .order_by(PostModel.created_at.desc())
            .limit(limit)
            .offset(offset)
        )
        return [self._to_entity(m) for m in result.scalars()]

    async def add(self, post: Post) -> None:
        model = PostModel(
            id=post.id,
            author_id=post.author_id,
            title=post.title,
            body=post.body,
            tags=post.tags,
            created_at=post.created_at,
            version=post.version,
        )
        self._session.add(model)
        await self._session.flush()

    async def save(self, post: Post) -> None:
        model = await self._session.get(PostModel, post.id)
        if model:
            model.title = post.title
            model.body = post.body
            model.tags = post.tags
            model.version = post.version
            await self._session.flush()

    def _to_entity(self, model: PostModel) -> Post:
        return Post(
            id=model.id,
            author_id=model.author_id,
            title=model.title,
            body=model.body,
            tags=model.tags,
            created_at=model.created_at,
            version=model.version,
        )
