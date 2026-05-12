from uuid import uuid4

from httpx import AsyncClient

from app.domain.entities.user import User


class TestCreateUser:
    async def test_create_user_returns_201(self, fresh_client: AsyncClient) -> None:
        response = await fresh_client.post("/users")

        assert response.status_code == 201
        body = response.json()
        assert "id" in body
        assert body["keycloak_sub"] == "sub-test-user"
        assert body["role"] == "user"
        assert body["is_active"] is True

    async def test_create_user_idempotent(self, fresh_client: AsyncClient) -> None:
        first = await fresh_client.post("/users")
        second = await fresh_client.post("/users")

        assert first.status_code == 201
        assert second.status_code == 200
        assert first.json()["id"] == second.json()["id"]

    async def test_create_user_requires_auth(self, unauthenticated_client: AsyncClient) -> None:
        response = await unauthenticated_client.post("/users")

        assert response.status_code == 401


class TestGetUser:
    async def test_get_user_returns_200(
        self, client: AsyncClient, test_user: User
    ) -> None:
        response = await client.get(f"/users/{test_user.id}")

        assert response.status_code == 200
        assert response.json()["id"] == str(test_user.id)

    async def test_get_user_not_found_returns_404(self, client: AsyncClient) -> None:
        missing_id = str(uuid4())

        response = await client.get(f"/users/{missing_id}")

        assert response.status_code == 404

    async def test_get_user_requires_auth(self, unauthenticated_client: AsyncClient) -> None:
        some_id = str(uuid4())

        response = await unauthenticated_client.get(f"/users/{some_id}")

        assert response.status_code == 401


class TestDeactivateUser:
    async def test_deactivate_user_returns_204(
        self, client: AsyncClient, test_user: User
    ) -> None:
        response = await client.delete(f"/users/{test_user.id}")

        assert response.status_code == 204

    async def test_deactivate_user_not_found_returns_404(self, client: AsyncClient) -> None:
        missing_id = str(uuid4())

        response = await client.delete(f"/users/{missing_id}")

        assert response.status_code == 404

    async def test_deactivate_user_requires_auth(self, unauthenticated_client: AsyncClient) -> None:
        some_id = str(uuid4())

        response = await unauthenticated_client.delete(f"/users/{some_id}")

        assert response.status_code == 401


class TestInactiveUserRestriction:
    async def test_get_user_inactive_returns_403(
        self, inactive_client: AsyncClient, inactive_user: User
    ) -> None:
        response = await inactive_client.get(f"/users/{inactive_user.id}")

        assert response.status_code == 403

    async def test_deactivate_user_inactive_returns_403(
        self, inactive_client: AsyncClient, inactive_user: User
    ) -> None:
        response = await inactive_client.delete(f"/users/{inactive_user.id}")

        assert response.status_code == 403
