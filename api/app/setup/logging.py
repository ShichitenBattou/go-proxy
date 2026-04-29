import logging
import sys

from pythonjsonlogger.json import JsonFormatter

from app.config import settings


def configure_logging() -> None:
    level = logging.DEBUG if settings.stage == "dev" else logging.INFO

    formatter = JsonFormatter(
        "%(asctime)s - %(levelname)s - %(message)s - %(pathname)s - %(lineno)d - %(process)d"
    )

    handler = logging.StreamHandler(sys.stdout)
    handler.setLevel(level)
    handler.setFormatter(formatter)

    root = logging.getLogger()
    root.setLevel(level)
    root.handlers.clear()
    root.addHandler(handler)

    for name in ("uvicorn", "uvicorn.access", "uvicorn.error"):
        uvicorn_logger = logging.getLogger(name)
        uvicorn_logger.handlers.clear()
        uvicorn_logger.propagate = True
