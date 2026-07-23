from fastapi import FastAPI

app = FastAPI(title="Sidus AI")


@app.get("/healthz")
def healthz() -> dict[str, str]:
    return {"service": "ai", "status": "ok"}
