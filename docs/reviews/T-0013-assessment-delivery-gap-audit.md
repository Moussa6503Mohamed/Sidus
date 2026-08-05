# T-0013: Assessment Delivery Gap Audit

## 1. Existing Capabilities (Usable for Learner Delivery)
- **Curriculum Taxonomy**: Single source of truth for syllabuses, topics, and objectives via the `catalogue` and `curriculum_map` packages.
- **Question Grounding**: Questions are strictly grounded to verified curriculum-map nodes, passing the rights/provenance gate automatically (T-0006/T-0007).
- **Immutable Rubrics**: Marking is reproducible. `question_rubric_versions` guarantees that criteria and max marks are immutable and locked to a specific question content revision.
- **Editorial UI/BFF**: Secure BFF architecture (`EditorialOperation` union) protecting Core routes from direct browser access.
- **Auth/Audit**: Clerk-based identity and strict RBAC (`editor`, `reviewer`, `admin`) are implemented natively.

## 2. Missing Pieces

### Question Delivery & Read Authorization
- No public or learner-facing route to fetch questions. Core's `GET /questions` strictly demands `question:read` (denied to learners).

### Response/Answer Payload Model
- No schema or API to represent a learner's submitted answer (whether text, selected option, or structured input).

### MCQ Options & Canonical Correct Answer
- Questions lack an `options` array for multiple-choice rendering.
- Rubrics only accept `criteria: RubricCriterion[]`. There is no representation for the canonical correct choice or exact-match string, preventing automated grading of deterministic questions.

### Attempt/Session Lifecycle & Persistence
- No `exam_sessions` or `exam_attempts` tables to record when a learner starts, pauses, or finishes an exam, or to persist answers.

### Marking Result & Rubric Traceability
- No schema to store the final mark, applied rubric version, or generated explanation linked to a learner's attempt.

### AI/FastAPI Responsibilities & Anthropic Boundaries
- AI service currently handles ingestion, but lacks `/mark` or `/explain` routes.
- Routing logic distinguishing Haiku (routine MCQ/short-answer) from Sonnet (complex structured marking) is undefined.

### Verified Explanation Cache Key
- Cache storage and lookup logic for `question + syllabus + rubric + language + explanation version` (as per D-0003) is missing.

### Learner Privacy, Ownership, & Rate Limits
- Missing row-level ownership logic to ensure learners can only read their own attempts.
- No rate-limiting on submission/marking endpoints to prevent AI abuse.

### Exam Mode Timer/Session Semantics
- Lack of server-side validation for timed exam sessions (e.g., rejecting late submissions).

### Frontend BFF Routes & UI States
- No `/api/learner/*` Next.js routes. Missing UI states for syllabus selection, exam player, timer, submission, and result review.

## 3. Hard Blockers Caused by Current Schema/API

1. **Learner Read Authorization**
   - **Evidence**: `services/core/internal/question/handlers.go:27-28` registers `GET /questions` guarded by `auth.PermReadQuestion`.
   - **Blocker**: Learners cannot fetch verified questions. `docs/question-rubric-model.md` explicitly states: "No learner has `question:read`, and no learner-facing question route exists."
2. **Missing MCQ Options in Model**
   - **Evidence**: `services/core/internal/question/model.go:68` (`Question` struct) and `packages/shared/src/contracts.ts:364`.
   - **Blocker**: The schema only holds `prompt` and `responseType`. A multiple-choice question cannot be delivered without options.
3. **Strict Rubric Decoder Rejects Correct Answers**
   - **Evidence**: `packages/shared/src/contracts.ts:403` (`RubricStructure`) and `services/core/internal/question/rubric.go` (as documented in `docs/question-rubric-model.md`).
   - **Blocker**: The strict token-based JSON decoder rejects any keys other than `criteria`. Adding `correctOption` or `exactMatch` currently fails with `400 invalid_rubric`.
4. **No Learner BFF Proxy Boundary**
   - **Evidence**: `apps/web/lib/editorial/core-proxy.ts` exposes only an `EditorialOperation` union.
   - **Blocker**: The browser has no path to communicate with Core for learner operations, maintaining the "no open proxy" rule but blocking learner UI.

## 4. Recommended Task Sequence to Evaluator MVP

1. **Task 1: Question & Rubric Schema Expansion**
   - **Goal**: Support MCQ options and deterministic correct answers.
   - **API/Schema Changes**: Add `options` (JSONB) to `questions`. Add `correct_answer` capability to `RubricStructure`. Update Core strict decoders.
   - **Tests**: Assert strict decoding passes for new fields; ensure backward compatibility.
   - **Release Gate**: Direct implementation.
2. **Task 2: Learner API & BFF Routes**
   - **Goal**: Enable secure delivery of verified questions to learners.
   - **API/Schema Changes**: Add `GET /learner/questions` in Core (returns `verified` status only, scrubs draft rubrics). Add `LearnerOperation` union in web BFF (`/api/learner/questions`).
   - **Tests**: Assert `learner` role is permitted; assert `editor/reviewer` draft fields are scrubbed.
   - **Release Gate**: Direct implementation.
3. **Task 3: Session & Attempt Persistence (Exam Mode Domain)**
   - **Goal**: Persist learner sessions, answers, and time-boundaries.
   - **API/Schema Changes**: Create `exam_sessions` and `exam_attempts` tables. Add endpoints to start session and submit answers. Enforce learner data isolation.
   - **Tests**: Attempt immutability post-submission; authorization barriers.
   - **Release Gate**: **Requires Owner Approval** (defines core Exam Mode domain rules).
4. **Task 4: AI Marking & Explanation Service**
   - **Goal**: Evaluate structured responses and generate caching keys.
   - **API/Schema Changes**: Add `/mark` endpoint in FastAPI. Implement Anthropic client (Haiku/Sonnet routing). Implement Redis/DB caching using `canonical explanation key`.
   - **Tests**: Cache hit/miss assertions; prompt injection safety; timeout fallbacks.
   - **Release Gate**: **Requires Owner Approval** (AI cost controls, prompt boundaries, and grading accuracy).
5. **Task 5: Frontend Learner Evaluator UI**
   - **Goal**: Provide the web app interface for Exam Mode.
   - **API/Schema Changes**: Build React components for syllabus selection, question rendering, submission timer, and explanation review.
   - **Tests**: State management; BFF proxy integration.
   - **Release Gate**: Direct implementation.

## 5. Decision Authority

- **Requires Owner Approval**:
  - **Attempt/Session Schema**: The semantic rules of "Exam Mode" (time limits, auto-submission, retries) define the product's core learning loop.
  - **AI Prompting & Routing**: Deciding the strict boundary of what Sonnet marks vs Haiku, and prompt structures, heavily impacts OPEX and assessment quality.
- **Implemented Directly**:
  - **Schema Extensions**: Adding `options` to questions and `correct_answer` to rubrics are natural extensions of the existing architecture.
  - **Learner BFF/Delivery**: Reusing the `EditorialOperation` pattern for a `LearnerOperation` union requires no new architectural paradigms.

## 6. Constraints Preserved

- **No Copyrighted Material**: No source PDFs, past-papers, or sample content are introduced or queried.
- **No Sample/Seeded Data**: No migrations will insert dummy questions or rubrics.
- **No AI Generation**: AI is strictly relegated to marking/explaining human-authored questions (Task 4); it does not generate question prompts.
- **No External Inspection**: Read-only audit; no production systems or external documents inspected.
- **No Mutations**: No DB migrations, runtime data changes, or code modifications were performed during this audit.
