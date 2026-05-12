from uuid import UUID


class DomainError(Exception):
    pass


class UserNotFoundError(DomainError):
    def __init__(self, user_id: UUID) -> None:
        super().__init__(f"User {user_id} not found")
        self.user_id = user_id


class PostNotFoundError(DomainError):
    def __init__(self, post_id: UUID) -> None:
        super().__init__(f"Post {post_id} not found")
        self.post_id = post_id
