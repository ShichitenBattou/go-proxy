import logging
import sys
from logging.handlers import TimedRotatingFileHandler

from pythonjsonlogger.json import JsonFormatter

from app.config import settings


def configure_logging() -> None:
    formatter = JsonFormatter(
        "%(asctime)s - %(levelname)s - %(message)s - %(pathname)s - %(lineno)d - %(process)d"
    )

    if settings.stage == "dev":
        logging.basicConfig(level=logging.DEBUG)

        sh = logging.StreamHandler(sys.stdout)
        sh.setLevel(logging.DEBUG)
        sh.setFormatter(formatter)
        logging.getLogger().addHandler(sh)

        fh = TimedRotatingFileHandler("api.log", when="midnight", interval=1, backupCount=7)
        fh.setLevel(logging.DEBUG)
        fh.setFormatter(formatter)
        logging.getLogger().addHandler(fh)

    else:
        logging.basicConfig(level=logging.INFO)

        fh = TimedRotatingFileHandler("api.log", when="midnight", interval=1, backupCount=7)
        fh.setLevel(logging.DEBUG)
        fh.setFormatter(formatter)
        logging.getLogger().addHandler(fh)

    for name in ("uvicorn", "uvicorn.access", "uvicorn.error"):
        uvicorn_logger = logging.getLogger(name)
        uvicorn_logger.handlers.clear()
        uvicorn_logger.propagate = True
