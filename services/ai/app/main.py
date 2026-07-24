from fastapi import FastAPI

from .ingestion_routes import router as ingestion_router

app = FastAPI(title="Sidus AI")

# Health stays public; the ingestion router is protected by the Clerk session dependency.
app.include_router(ingestion_router)


@app.get("/healthz")
def healthz() -> dict[str, str]:
    return {"service": "ai", "status": "ok"}
