"""Sonnet provider interface (T-0033).

``SonnetProvider`` is the seam between the job orchestrator and whatever
backend answers a request. ``get_provider()`` is the only production entry
point and is fail-closed by construction: T-0033 delivers the adapter's
interface, schemas, job lifecycle, and quality gate without a live Anthropic
integration (see the boundaries in docs/tasks/active.md). A future task wires
a real ANTHROPIC_API_KEY-backed implementation in here; until then this
always returns None, and every caller must treat that as "adapter
unavailable" and fail closed rather than fabricate a result.
"""

from __future__ import annotations

from typing import Any, Optional, Protocol, runtime_checkable

from .schemas import SonnetRequest, SonnetResult


class ProviderError(Exception):
    """Raised when a provider call fails or returns an unusable response."""


@runtime_checkable
class SonnetProvider(Protocol):
    def generate(self, request: SonnetRequest) -> SonnetResult: ...


@runtime_checkable
class PrivateMarkingProvider(Protocol):
    """Optional service-only extension.

    This receives private learner answer/rubric context in memory for a single service request;
    it is never serialized into the public SonnetRequest, durable job JSON, traces, or APIs.
    """
    def generate_marking(self, request: SonnetRequest, private_context: dict[str, Any]) -> SonnetResult: ...


def get_provider() -> Optional[SonnetProvider]:
    """Resolve the production Sonnet provider, or None if unavailable.

    Always None in this build. Overridable in tests via
    ``app.dependency_overrides[get_provider]``, mirroring the existing
    ``get_authenticator`` fail-closed pattern in app.auth.
    """
    return None
