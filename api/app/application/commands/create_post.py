from dataclasses import dataclass
from datetime import UTC, datetime
from uuid import UUID, uuid4

from app.domain.entities.post import Post
from app.domain.entities.user import UserRole
from app.domain.exceptions import UserNotAuthorizedError, UserNotFoundError
from app.domain.ports.post_repository import PostRepository
from app.domain.ports.user_repository import UserRepository


@dataclass
class CreatePostCommand:
    author_id: UUID
    title: str
    body: str
    tags: list[str]


class CreatePostInteractor:
    def __init__(
        self,
        post_repository: PostRepository,
        user_repository: UserRepository,
    ) -> None:
        self._post_repository = post_repository
        self._user_repository = user_repository

    async def execute(self, command: CreatePostCommand) -> Post:
        author = await self._user_repository.get_by_id(command.author_id)
        if not author:
            raise UserNotFoundError(command.author_id)

        # システム管理者以外のすべてのユーザーは投稿可能
        if author.role == UserRole.ADMIN:
            raise UserNotAuthorizedError("Administrators cannot create posts")
        if not author.is_active:
            raise UserNotAuthorizedError("Inactive users cannot create posts")

        post = Post(
            id=uuid4(),
            author_id=command.author_id,
            title=command.title,
            body=command.body,
            tags=command.tags,
            created_at=datetime.now(UTC),
        )
        await self._post_repository.add(post)
        return post
