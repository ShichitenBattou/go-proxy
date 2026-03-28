FROM python:3.14.3 AS builder

COPY --from=ghcr.io/astral-sh/uv:0.10.10 /uv /usr/local/bin/uv

WORKDIR /app

COPY pyproject.toml uv.lock ./
ENV UV_PROJECT_ENVIRONMENT=/venv
RUN uv sync --frozen --no-dev


FROM builder AS builder-dev

RUN uv sync --frozen


FROM python:3.14.3 AS dev

COPY --from=builder-dev /venv /venv

ENV PATH="/venv/bin:$PATH"

WORKDIR /app

CMD ["python", "-m", "uvicorn", "main:app", "--reload", "--host", "0.0.0.0", "--port", "8081"]


FROM python:3.14.3-slim AS production

WORKDIR /app

COPY --from=builder /venv /venv
COPY main.py ./
COPY app/ ./app/
COPY alembic/ ./alembic/
COPY alembic.ini ./

ENV PATH="/venv/bin:$PATH"

ENTRYPOINT ["python", "-m", "uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8081"]
