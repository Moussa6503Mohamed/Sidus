from fastapi import FastAPI

from .ingestion_routes import router as ingestion_router
from .sonnet.routes import router as sonnet_router

app = FastAPI(title="Sidus AI")

# Health stays public; the ingestion and sonnet routers are protected by the Clerk
# session dependency.
app.include_router(ingestion_router)
app.include_router(sonnet_router)


@app.get("/healthz")
def healthz() -> dict[str, str]:
    return {"service": "ai", "status": "ok"}
