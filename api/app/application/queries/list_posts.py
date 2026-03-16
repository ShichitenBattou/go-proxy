from dataclasses import dataclass
from uuid import UUID

from app.domain.entities.post import Post
from app.domain.ports.post_repository import PostRepository


@dataclass
class ListPostsQuery:
    author_id: UUID | None = None
    limit: int = 20
    offset: int = 0


class ListPostsInteractor:
    def __init__(self, post_repository: PostRepository) -> None:
        self._post_repository = post_repository

    async def execute(self, query: ListPostsQuery) -> list[Post]:
        if query.author_id:
            return await self._post_repository.list_by_author(query.author_id)
        return await self._post_repository.list_all(query.limit, query.offset)
