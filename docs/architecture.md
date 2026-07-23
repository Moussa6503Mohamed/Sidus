# Architecture foundation

## Service boundaries

`apps/web` serves Next.js PWA experience: Observatory dashboard, theme system, distraction-free Exam Mode.

`services/core` owns high-traffic API, user/session progression, attempts, permissions, and canonical cache lookup.

`services/ai` owns OCR, ingestion, marking, and AI orchestration. It calls Anthropic only: Haiku for routine tasks; Sonnet for complex marking.

`packages/shared` owns stable contracts shared by frontend and service clients.

## Data direction

PostgreSQL, Redis, object storage, and OpenSearch are planned; not provisioned in this foundation.

Canonical explanation cache key:

`question + syllabus + rubric + language + explanation version`

Core must return verified cached explanation before AI generation. Identical verified explanation must not regenerate.

## Scale path

Private beta: about 10 users. Long-term target: 100,000 concurrent users. Keep API services stateless, move heavy ingestion/marking to jobs, and isolate object storage behind service APIs.
