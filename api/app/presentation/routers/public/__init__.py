from fastapi import APIRouter

from app.presentation.routers.public import posts

router = APIRouter(prefix="/public", tags=["public"])
router.include_router(posts.router)
