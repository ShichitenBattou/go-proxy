from uuid import uuid4

from httpx import AsyncClient

from tests.presentation.conftest import InMemoryUserRepository


class TestCreateUser:
    async def test_create_user_returns_201(self, client: AsyncClient) -> None:
        response = await client.post("/users")

        assert response.status_code == 201
        body = response.json()
        assert "id" in body
        assert body["keycloak_sub"] == "sub-test-user"
        assert body["role"] == "user"
        assert body["is_active"] is True

    async def test_create_user_idempotent_returns_201(self, client: AsyncClient) -> None:
        first = await client.post("/users")
        second = await client.post("/users")

        assert first.status_code == 201
        assert second.status_code == 201
        assert first.json()["id"] == second.json()["id"]

    async def test_create_user_requires_auth(self, unauthenticated_client: AsyncClient) -> None:
        response = await unauthenticated_client.post("/users")

        assert response.status_code == 401


class TestGetUser:
    async def test_get_user_returns_200(
        self, client: AsyncClient, user_repo: InMemoryUserRepository
    ) -> None:
        # Create a user first, then retrieve it.
        create_resp = await client.post("/users", json={"keycloak_sub": "get-sub"})
        user_id = create_resp.json()["id"]

        response = await client.get(f"/users/{user_id}")

        assert response.status_code == 200
        assert response.json()["id"] == user_id

    async def test_get_user_not_found_returns_404(self, client: AsyncClient) -> None:
        missing_id = str(uuid4())

        response = await client.get(f"/users/{missing_id}")

        assert response.status_code == 404

    async def test_get_user_requires_auth(self, unauthenticated_client: AsyncClient) -> None:
        some_id = str(uuid4())

        response = await unauthenticated_client.get(f"/users/{some_id}")

        assert response.status_code == 401


class TestDeactivateUser:
    async def test_deactivate_user_returns_204(self, client: AsyncClient) -> None:
        # Create a user first, then deactivate it.
        create_resp = await client.post("/users", json={"keycloak_sub": "deactivate-sub"})
        assert create_resp.status_code == 201
        user_id = create_resp.json()["id"]

        response = await client.delete(f"/users/{user_id}")

        assert response.status_code == 204

    async def test_deactivate_user_not_found_returns_404(self, client: AsyncClient) -> None:
        missing_id = str(uuid4())

        response = await client.delete(f"/users/{missing_id}")

        assert response.status_code == 404

    async def test_deactivate_user_requires_auth(self, unauthenticated_client: AsyncClient) -> None:
        some_id = str(uuid4())

        response = await unauthenticated_client.delete(f"/users/{some_id}")

        assert response.status_code == 401
