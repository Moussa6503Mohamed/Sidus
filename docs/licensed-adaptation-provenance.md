# Licensed-adaptation question provenance

## Policy

Sidus may create and deliver a licensed adaptation only when a human-approved content source has
written evidence covering every applicable use:

- adaptation;
- digital delivery;
- intended learner audience;
- marking feedback; and
- AI processing, when AI processing will occur.

Licence facts are evidence entered by a human. Sidus code, agents, tests, migrations, and AI must
not invent or infer a licence reference, rights owner, date, audience, territory, attribution, or
permission. Approval means a human reviewer has checked written evidence; a non-blank field is not
machine proof that a licence is legally sufficient.

Source material is outside this workflow. Do not upload, commit, copy, extract, seed, log, return,
OCR, parse, transform, generate, or recreate any PDF, coursebook, past paper, question wording,
image, diagram, answer, or mark scheme.

## Human approval workflow

1. Human editor creates or updates metadata-only `content_source` record. Human enters actual
   rights evidence in existing owner, source identity/hash, licence reference, permitted-use, and
   allowed-audience fields. Written evidence must exist before approval.
2. Human links source to correct active catalogue syllabus through existing source workflow.
   Sidus never guesses catalogue link from title, URL, filename, or source content.
3. Human reviewer checks written licence evidence against adaptation, digital delivery, learner
   audience, marking feedback, and AI-processing requirements. Missing or ambiguous coverage means
   source must not be approved.
4. Reviewer approves source only after rights fields are complete and evidence is sufficient.
   Approval actor and decision remain existing human-authenticated source records.
5. Editor starts question create flow and explicitly selects one origin:
   - `original`: independently authored question; no external source or locator accepted. Existing
     verified curriculum-node/source grounding remains mandatory.
   - `licensed_adaptation`: editor selects approved syllabus-linked source and enters metadata-only
     locator. Locator may identify paper/session/question reference or book chapter/page, but must
     never contain copied source wording or content.
6. Core rechecks source exists, status is `approved`, catalogue link equals question syllabus, and
   every existing approval-required rights field is non-blank. Core then atomically creates draft
   question, immutable provenance, question audit event, and names-only provenance event.
7. Reviewer verifies rubric and question through existing workflow. Question verification reruns
   curriculum grounding and licensed provenance source gate. Failure leaves question draft.
8. Learner question list/get, attempt creation, and attempt feedback submission rerun licensed
   gate. Rejected, expired, unlinked, foreign-linked, missing-provenance, or rights-incomplete
   licensed question is absent/not found. No latest or fallback source is selected.

## Data model

`questions.origin_type` is nullable only for historical pre-T-0018 rows. Migration performs no
backfill; historical rows stay unclassified and keep existing behavior. Every new create request
must supply exact case-sensitive `original` or `licensed_adaptation`.

`question_provenance` exists only for licensed adaptations and stores metadata only:

- `question_id`;
- `content_source_id`;
- `source_locator`;
- `origin_type` fixed to `licensed_adaptation`;
- authenticated `verified_actor_id`;
- `verified_at`; and
- `created_at`.

Composite foreign key proves provenance origin matches question origin. Database triggers reject
question-origin changes and provenance update/delete. API PATCH allowlist also rejects origin,
source, locator, and provenance fields before store call.

`question_provenance_events` is append-only. Event stores authenticated actor, timestamps, event
name, and changed field names only. It never stores locator value, rights value, licence reference,
question content, or source content.

## Contracts and visibility

Editorial `Question` includes nullable historical `originType` and nullable licensed
`provenance`. Create contract is discriminated union: original forbids external provenance fields;
licensed adaptation requires both source id and non-blank locator. Unknown keys, case variants,
trailing JSON, caller-supplied actors, malformed identifiers, and foreign identifiers fail with
stable safe errors before mutation.

Learner types remain handwritten and separate. Learner question, attempt, and result responses do
not contain origin type, provenance, source id, locator, licence reference, rights fields, events,
actors, or provenance timestamps.

Web editorial create flow lists only approved sources and additionally filters for selected
syllabus link and non-blank required rights metadata. UI provides origin selection, source picker,
locator input, and rights warning. It provides no source upload, preview, copied-text field, PDF,
OCR, extraction, or AI tooling. Existing closed BFF operations are reused; browser cannot choose
an upstream URL or widen method/route allowlist.

## Regression behavior

Licensed source regression blocks further question/rubric writes and learner delivery. Question
stays stored for editorial history; origin and provenance remain immutable. Existing canonical
rubric selection, content revision, current-rubric checks, attempt ownership, pinned rubric and
revision, deterministic marking, curriculum-node grounding, and names-only audit guarantees remain
in force. Original and historical unclassified questions do not gain or resolve external
provenance.
