import logging
from http import HTTPStatus
from socket import gaierror

from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse

from app.application.exceptions import UserNotAuthorizedError

logger = logging.getLogger(__name__)


def add_exception_handler(app: FastAPI) -> None:
    # FastAPI guarantees type safety at runtime by dispatching to the correct handler
    app.add_exception_handler(UserNotAuthorizedError, handle_user_not_authorized_error)  # type: ignore[arg-type]
    app.add_exception_handler(gaierror, handle_gaierror)  # type: ignore[arg-type]
    app.add_exception_handler(Exception, handle_exception)


async def handle_user_not_authorized_error(
    request: Request, exc: UserNotAuthorizedError
) -> JSONResponse:
    logger.error(f"User not authorized error occurred: {exc}")
    return JSONResponse(
        status_code=HTTPStatus.FORBIDDEN,
        content={"detail": "You do not have permission to perform this action."},
    )


async def handle_gaierror(request: Request, exc: gaierror) -> JSONResponse:
    logger.error(f"gaierror occurred: {exc}")
    return JSONResponse(
        status_code=HTTPStatus.SERVICE_UNAVAILABLE,
        content={"detail": "Network error occurred. Please try again later."},
    )


async def handle_exception(request: Request, exc: Exception) -> JSONResponse:
    logger.error(f"Unhandled exception: {exc}")
    return JSONResponse(
        status_code=HTTPStatus.INTERNAL_SERVER_ERROR,
        content={"detail": "An unexpected error occurred. Please try again later."},
    )
