from dataclasses import dataclass
from uuid import UUID

from app.domain.entities.post import Post
from app.domain.exceptions import PostNotFoundError
from app.domain.ports.post_repository import PostRepository


@dataclass
class GetPostQuery:
    post_id: UUID


class GetPostInteractor:
    def __init__(self, post_repository: PostRepository) -> None:
        self._post_repository = post_repository

    async def execute(self, query: GetPostQuery) -> Post:
        post = await self._post_repository.get_by_id(query.post_id)
        if post is None:
            raise PostNotFoundError(query.post_id)
        return post
