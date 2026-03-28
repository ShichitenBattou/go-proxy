from uuid import uuid4

from httpx import AsyncClient

from tests.presentation.conftest import InMemoryUserRepository


class TestCreateUser:
    async def test_create_user_returns_201(self, client: AsyncClient) -> None:
        response = await client.post("/users/", json={"keycloak_sub": "new-sub-001"})

        assert response.status_code == 201
        body = response.json()
        assert "id" in body
        assert body["keycloak_sub"] == "new-sub-001"
        assert body["role"] == "user"
        assert body["is_active"] is True

    async def test_create_user_idempotent_returns_201(self, client: AsyncClient) -> None:
        payload = {"keycloak_sub": "idempotent-sub"}

        first = await client.post("/users/", json=payload)
        second = await client.post("/users/", json=payload)

        assert first.status_code == 201
        assert second.status_code == 201
        assert first.json()["id"] == second.json()["id"]


class TestGetUser:
    async def test_get_user_returns_200(
        self, client: AsyncClient, user_repo: InMemoryUserRepository
    ) -> None:
        # Create a user first, then retrieve it.
        create_resp = await client.post("/users/", json={"keycloak_sub": "get-sub"})
        user_id = create_resp.json()["id"]

        response = await client.get(f"/users/{user_id}")

        assert response.status_code == 200
        assert response.json()["id"] == user_id

    async def test_get_user_not_found_returns_404(self, client: AsyncClient) -> None:
        missing_id = str(uuid4())

        response = await client.get(f"/users/{missing_id}")

        assert response.status_code == 404
