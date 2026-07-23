# Content provenance register

## Rule

Repository stores metadata, source links, hashes, rights decisions, independently authored content, and approved assets only. No copyrighted source PDFs, extracted text, diagrams, screenshots, or lightly rewritten questions.

## Sources

| Source ID | Title | Source URL | Version / exam years | Retrieved | Stored locally | Approved use |
| --- | --- | --- | --- | --- | --- | --- |
| CAM-0610-2026 | Cambridge IGCSE Biology 0610 syllabus | https://www.cambridgeinternational.org/Images/697203-2026-2028-syllabus.pdf | Version 2; 2026–2028 | 2026-07-23 | No | Link, version metadata, human-reviewed topic/objective mapping only |
| CAM-5090-2026 | Cambridge O Level Biology 5090 syllabus | https://www.cambridgeinternational.org/Images/697330-2026-2028-syllabus.pdf | Version 4; 2026–2028 | 2026-07-23 | No | Link, version metadata, human-reviewed topic/objective mapping only |

## Rights gate

Before ingestion or publication, record: owner, licence/permission, source URI, file hash, permitted transformation, permitted audience, expiry, reviewer, approval date.

Status now: no source material approved for ingestion. Build original content from editor-reviewed learning objectives only.

This gate is now enforced in code (T-0001): the `content_sources` / `content_source_reviews` tables in `services/core` hold this same information, approval is blocked until all rights fields above are present, and `services/ai` rejects ingestion for every source that is not `approved`. CAM-0610-2026 and CAM-5090-2026 are seeded as `pending` metadata rows with owner/hash/licence-reference/allowed-audience left unset, matching "Stored locally: No" above — they cannot be approved until a reviewer supplies and confirms those fields.
