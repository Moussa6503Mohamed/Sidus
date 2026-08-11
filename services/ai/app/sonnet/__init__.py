"""Sonnet adapter and automated quality gate (T-0033).

Provider interface, strict request/result schemas, durable job lifecycle with
retry/withhold, and deterministic rubric/consistency validation. No live
Anthropic call, API key, source PDF, OCR, or extraction is involved — see
docs/tasks/active.md T-0033 for the full boundary.
"""
