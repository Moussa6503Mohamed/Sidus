"""Protected foundation for future ingestion endpoints.

This wires the Clerk session dependency in front of the (future) ingestion surface so that
every ingestion route is authenticated by construction. It deliberately performs NO OCR and
NO content ingestion — the actual rights/provenance decision stays with
`app.ingestion.guard_ingestion`, and no source material is touched here.
"""

from __future__ import annotations

from fastapi import APIRouter, Depends

from .auth import Principal, require_clerk_session

router = APIRouter(prefix="/ingestion", tags=["ingestion"])


@router.get("/status")
def ingestion_status(principal: Principal = Depends(require_clerk_session)) -> dict[str, str]:
    """Report that the caller holds a valid Clerk session.

    Placeholder guard for the future ingestion surface: it authenticates the caller and
    returns the verified subject/role. It does not ingest, fetch, OCR, or store any source
    material.
    """
    return {
        "status": "authenticated",
        "subject": principal.subject,
        "role": principal.role,
    }
