# Private upload intake

Admin-only local intake for private PDFs.

1. Admin uploads one PDF (maximum 25 MiB). BFF and Core validate filename, media type, and `%PDF-` signature. Core writes bytes only to `SIDUS_PRIVATE_UPLOAD_DIR`.
2. Core stores metadata, SHA-256, opaque object reference, lifecycle state, and immutable names-only audit event. No endpoint returns bytes.
3. Upload begins `quarantined`. A trusted scanner adapter/operator may attest `scan_clean`.
4. Admin selects an approved, catalogue-linked source and queues a metadata-only `sonnet-review-v1` job. No model is called in T-0030.
5. `deletion_requested` is an auditable retention state. Physical deletion needs a future worker; it is not silently performed by a web request.

Private path must be outside repository, for example `D:\Sidus-private-content\uploads`. Never use a Git or browser-readable static folder.

AI contract in `services/ai/app/review_intake.py` accepts opaque IDs and metadata only. Future Sonnet results require human review and cannot auto-publish questions.
