from dataclasses import dataclass, field
from datetime import datetime
from uuid import UUID


@dataclass
class Post:
    id: UUID
    author_id: UUID
    title: str
    body: str
    tags: list[str]
    created_at: datetime
    version: int = 1
