from dataclasses import dataclass, field
from enum import StrEnum
from uuid import UUID


class UserRole(StrEnum):
    ADMIN = "admin"
    USER = "user"


@dataclass
class User:
    id: UUID
    keycloak_sub: str  # Keycloak の一意識別子 (sub クレーム)
    role: UserRole = field(default=UserRole.USER)
    is_active: bool = True
