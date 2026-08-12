// A syllabus code is an opaque string (e.g. "0610", "5090"). It is NOT a fixed two-value
// union: the curriculum catalogue (see Curriculum catalogue types below) is the authority for
// which codes exist, so new approved syllabuses are onboarded as data, not code changes.
export type SyllabusCode = string;

export interface CanonicalExplanationKey {
  question: string;
  syllabus: SyllabusCode;
  rubric: string;
  language: string;
  explanationVersion: string;
}

export interface HealthResponse { service: string; status: "ok"; }

// --- Content rights/provenance gate (T-0001) ---
// Mirrors services/core/internal/contentsource and services/ai/app/content_sources.py.
// Metadata only: never the source material itself (no PDFs, extracts, diagrams, etc.).

export type ContentSourceStatus = "pending" | "approved" | "rejected" | "expired";

export interface ContentSource {
  id: string;
  title: string;
  owner: string | null;
  /**
   * An external `http`/`https` URL, or the approved private source-reference URI for a
   * licensed source bundle not publicly resolvable (D-0021):
   * `sidus-private://licensed/cambridge-international/9700/{session}/{component}`, session
   * exactly `[msw]\d{2}`, component exactly two digits. Core rejects any other private scheme
   * or malformed variant before storage. The URI is metadata identity only — it is never
   * dereferenced, fetched, or exposed to learners.
   */
  sourceUrl: string;
  sourceHash: string | null;
  licenceReference: string | null;
  permittedUse: string | null;
  allowedAudience: string | null;
  syllabusCode: SyllabusCode | null;
  /** Registry-resolved catalogue syllabus id for syllabusCode; null when unassociated. */
  catalogueSyllabusId: string | null;
  status: ContentSourceStatus;
  createdAt: string;
  updatedAt: string;
}

export type ReviewDecision = Extract<ContentSourceStatus, "approved" | "rejected">;

export interface ContentSourceReview {
  id: string;
  contentSourceId: string;
  decision: ReviewDecision;
  reviewerId: string;
  decisionDate: string;
  reason: string | null;
  createdAt: string;
}

/** Fields required (non-empty) on a ContentSource before it can be approved. */
export const REQUIRED_APPROVAL_FIELDS = [
  "owner",
  "title",
  "sourceUrl",
  "sourceHash",
  "licenceReference",
  "permittedUse",
  "allowedAudience",
] as const;

export type RequiredApprovalField = (typeof REQUIRED_APPROVAL_FIELDS)[number];

export interface CreateContentSourceRequest {
  title: string;
  /** See {@link ContentSource.sourceUrl}: external URL or approved private reference URI. */
  sourceUrl: string;
  owner?: string;
  sourceHash?: string;
  licenceReference?: string;
  permittedUse?: string;
  allowedAudience?: string;
  syllabusCode?: SyllabusCode;
}

// Approve/reject/update requests carry NO caller-supplied identity. The reviewer/actor is
// derived server-side from the verified Clerk session subject (see AuthenticatedRequest).
export interface ApproveContentSourceRequest {
  decisionDate?: string;
}

export interface RejectContentSourceRequest {
  reason: string;
  decisionDate?: string;
}

export interface MissingApprovalFieldsError {
  error: "missing_required_fields";
  missing: RequiredApprovalField[];
}

// --- Pending source metadata update + audit (T-0002) ---
// PATCH /content-sources/{id} lets a curator complete metadata on a pending source.
// It never approves and never stores source material or field values in the audit trail.

/** Fields a PATCH may change on a pending ContentSource. */
export const UPDATABLE_CONTENT_SOURCE_FIELDS = [
  "title",
  "owner",
  "sourceUrl",
  "sourceHash",
  "licenceReference",
  "permittedUse",
  "allowedAudience",
  "syllabusCode",
] as const;

export type UpdatableContentSourceField =
  (typeof UPDATABLE_CONTENT_SOURCE_FIELDS)[number];

/**
 * Update a pending content source. At least one updatable field must be supplied; supplied
 * fields must be non-empty. Only pending sources may be updated. The actor is derived from
 * the verified Clerk session subject — never a request-body field.
 */
export interface UpdateContentSourceRequest {
  title?: string;
  owner?: string;
  /** See {@link ContentSource.sourceUrl}: external URL or approved private reference URI. */
  sourceUrl?: string;
  sourceHash?: string;
  licenceReference?: string;
  permittedUse?: string;
  allowedAudience?: string;
  syllabusCode?: SyllabusCode;
}

export type ContentSourceEventType = "metadata_updated";

/**
 * Immutable audit record of a metadata change. Records which fields changed (names only)
 * and who changed them — never the field values, and never any source material.
 */
export interface ContentSourceEvent {
  id: string;
  contentSourceId: string;
  eventType: ContentSourceEventType;
  actorId: string;
  eventTime: string;
  changedFields: UpdatableContentSourceField[];
  createdAt: string;
}

// --- Authentication and roles (T-0003) ---
// Clerk owns authentication; Sidus Core owns authorization. Every content-source request
// carries a verified Clerk session as `Authorization: Bearer <token>`; the authenticated
// subject (never a body field) becomes the audit actor/reviewer. Authorization derives from
// the verified `sidus_role` session claim. Mirrors services/core/internal/auth and
// services/ai/app/auth.py.

/** Sidus authorization roles, sourced from the verified `sidus_role` session claim. */
export const SIDUS_ROLES = ["learner", "editor", "reviewer", "admin"] as const;

/** A known Sidus role. A missing/unrecognized claim is denied by default (no access). */
export type SidusRole = (typeof SIDUS_ROLES)[number];

/** The Clerk session claim name that carries the Sidus role. */
export const SIDUS_ROLE_CLAIM = "sidus_role";

/**
 * Every content-source request must be authenticated with a Clerk session bearer token in
 * the `Authorization` header. There is no caller-supplied actor/reviewer field.
 */
export interface AuthenticatedRequestHeaders {
  Authorization: `Bearer ${string}`;
}

// --- Curriculum catalogue (T-0004) ---
// Metadata-only registry of subjects and syllabuses. Core is the single authority. Never
// holds source material, questions, objectives, assessment rules, or rights claims. Mirrors
// services/core/internal/catalogue. Reads require content_catalogue:read (editor/reviewer/
// admin); create/change require content_catalogue:manage (admin only). Learner and unknown
// roles are denied.

/** Lifecycle state of a catalogue syllabus. */
export type SyllabusLifecycleStatus = "draft" | "active" | "retired";

/** A normalized subject (e.g. "Biology"). Referenced by id, never by free-text on requests. */
export interface Subject {
  id: string;
  name: string;
  createdAt: string;
  updatedAt: string;
}

/** A curriculum-catalogue syllabus record: identity metadata only. */
export interface Syllabus {
  id: string;
  board: string;
  syllabusCode: SyllabusCode;
  subjectId: string;
  qualification: string;
  /** Tier/route within a syllabus (e.g. "Extended"); null when not applicable. */
  track: string | null;
  displayName: string;
  /** Curriculum year/edition; null unless explicitly known (never inferred). */
  curriculumYear: string | null;
  status: SyllabusLifecycleStatus;
  createdAt: string;
  updatedAt: string;
}

export interface CreateSubjectRequest {
  name: string;
}

/** Create a catalogue syllabus (admin only). track/curriculumYear/status are optional. */
export interface CreateSyllabusRequest {
  board: string;
  syllabusCode: SyllabusCode;
  subjectId: string;
  qualification: string;
  track?: string;
  displayName: string;
  curriculumYear?: string;
  status?: SyllabusLifecycleStatus;
}

/**
 * Update catalogue metadata on a syllabus (admin only). At least one field must be supplied;
 * supplied fields must be non-empty. The actor is derived from the verified Clerk subject.
 */
export interface UpdateSyllabusRequest {
  board?: string;
  syllabusCode?: SyllabusCode;
  subjectId?: string;
  qualification?: string;
  track?: string;
  displayName?: string;
  curriculumYear?: string;
  status?: SyllabusLifecycleStatus;
}

export type SyllabusEventType = "syllabus_created" | "syllabus_updated";

/**
 * Immutable audit record of a catalogue mutation. Records which fields were set/changed
 * (names only) and who changed them — never the field values, and never any source material.
 */
export interface SyllabusEvent {
  id: string;
  syllabusId: string;
  eventType: SyllabusEventType;
  actorId: string;
  eventTime: string;
  changedFields: string[];
  createdAt: string;
}

// --- Curriculum map (T-0006) ---
// Metadata-only infrastructure for topic maps, learning objectives, practical skills, and
// assessment rules. Core is the sole authority — it enforces the source gate (approved,
// syllabus-matched content source required) before any write. Never holds syllabus text,
// objective wording, topic labels, assessment text, questions, or mark schemes; `label` is a
// placeholder editorial field for a future private, approved authoring workflow. No initial
// map data is seeded. Mirrors services/core/internal/curriculummap. Reads (verified nodes
// only) require curriculum_map:read (editor/reviewer/admin); draft create/update require
// curriculum_map:create (editor/reviewer/admin); verify/retire require curriculum_map:verify
// (reviewer/admin). Learner and unknown roles are denied.

/** Kind of curriculum-map node. */
export type CurriculumMapNodeKind =
  | "topic"
  | "objective"
  | "practical_skill"
  | "assessment_rule";

/** Lifecycle state of a curriculum-map node. */
export type CurriculumMapNodeStatus = "draft" | "verified" | "retired";

/** A curriculum-map node: hierarchy position, editorial identity, lifecycle, and the approved
 * content source it is grounded in. Never carries syllabus/objective/assessment text. */
export interface CurriculumMapNode {
  id: string;
  syllabusId: string;
  /** Parent node id within the same syllabus; null for a root node. */
  parentNodeId: string | null;
  nodeKind: CurriculumMapNodeKind;
  /** Stable editorial reference/code, unique within its syllabus. */
  nodeCode: string;
  /** Editorial label/summary — placeholder for the future private approved workflow. */
  label: string;
  status: CurriculumMapNodeStatus;
  /** Approved content_sources id this node is grounded in; catalogueSyllabusId must match
   * syllabusId (enforced server-side before every write). */
  contentSourceId: string;
  /** Optional reference metadata pointing at the approved source (e.g. a section label) —
   * never the source content itself. */
  sourceLocator: string | null;
  createdAt: string;
  updatedAt: string;
}

/** Create a draft curriculum-map node (editor/reviewer/admin). The syllabus a node belongs to
 * is not changeable after creation. */
export interface CreateCurriculumMapNodeRequest {
  syllabusId: string;
  parentNodeId?: string;
  nodeKind: CurriculumMapNodeKind;
  nodeCode: string;
  label: string;
  contentSourceId: string;
  sourceLocator?: string;
}

/**
 * Update a draft curriculum-map node (editor/reviewer/admin). Only a node whose status is
 * draft may be updated. At least one field must be supplied; supplied non-nullable fields must
 * be non-empty. `parentNodeId`/`sourceLocator` may be explicitly set to `null` to clear them —
 * omit the key entirely to leave it unchanged.
 */
export interface UpdateCurriculumMapNodeRequest {
  parentNodeId?: string | null;
  nodeKind?: CurriculumMapNodeKind;
  nodeCode?: string;
  label?: string;
  contentSourceId?: string;
  sourceLocator?: string | null;
}

export type CurriculumMapEventType =
  | "node_created"
  | "node_updated"
  | "node_verified"
  | "node_retired";

/**
 * Immutable audit record of a curriculum-map node mutation. Records which fields changed
 * (names only) and who changed them — never field values, and never any source material.
 */
export interface CurriculumMapEvent {
  id: string;
  nodeId: string;
  eventType: CurriculumMapEventType;
  actorId: string;
  eventTime: string;
  changedFields: string[];
  createdAt: string;
}

// --- Editorial questions, licensed provenance, and versioned rubrics (T-0007/T-0018) ---
// Private infrastructure. Question prompts and rubric structures are authored at runtime by a
// private approved editorial workflow and never committed as repository content. Licensed
// adaptations carry metadata-only provenance; source material is never copied, uploaded,
// extracted, or returned. Core is
// the sole authority: on every write it re-validates that the question's curriculum-map node is
// verified, belongs to the question's syllabus, and that the node's content source still passes
// the T-0006 source gate. Mirrors services/core/internal/question. Editorial reads of questions
// in any lifecycle state require question:read; draft create/update and draft rubric-version create require
// question:create; verify/retire require question:verify; listing rubric versions requires
// question_rubric:read. Learner and unknown roles are denied. No AI generation, OCR, ingestion,
// or question derivation is involved.

/** Shape of answer a question expects. */
export type QuestionResponseType =
  | "multiple_choice"
  | "multi_select"
  | "exact_short_answer"
  | "numeric_answer"
  | "short_answer"
  | "structured_response"
  | "essay";

/** Original editorial MCQ option. Stable id anchors rubric answer keys across label edits. */
export interface MultipleChoiceOption {
  id: string;
  label: string;
}

/** Lifecycle state of a question. Retired questions remain visible only to editorial readers. */
export type QuestionStatus = "draft" | "verified" | "retired";

/** Editorial-only origin. Null exists only on historical pre-T-0018 Question rows. */
export type QuestionOriginType = "original" | "licensed_adaptation";

/** Immutable metadata-only licensed provenance. Never use this type on learner surfaces. */
export interface QuestionProvenance {
  questionId: string;
  contentSourceId: string;
  sourceLocator: string;
  originType: "licensed_adaptation";
  /** Authenticated Clerk subject recorded by Core, never supplied by request body. */
  verifiedActorId: string;
  verifiedAt: string;
  createdAt: string;
}

/** Lifecycle state of a rubric version. A version is superseded by a new version, never retired. */
export type RubricVersionStatus = "draft" | "verified";

/** An original question record. */
export interface Question {
  id: string;
  syllabusId: string;
  /** Verified curriculum-map node this question traces to; must belong to syllabusId. */
  curriculumMapNodeId: string;
  responseType: QuestionResponseType;
  /** Opaque language tag (e.g. "en"). */
  language: string;
  /** ORIGINAL question body, authored privately at runtime. */
  prompt: string;
  /** Present only for multiple_choice; pre-T-0013 rows can return null until edited. */
  options: MultipleChoiceOption[] | null;
  /** Null only for historical rows. Editorial-only; learner projections omit it. */
  originType: QuestionOriginType | null;
  /** Present exactly for licensed adaptations. Editorial-only. */
  provenance: QuestionProvenance | null;
  status: QuestionStatus;
  /** Explicit reviewer-selected rubric row id; null for drafts and unrepaired historical rows. */
  canonicalRubricVersionId: string | null;
  /**
   * Revision of the question's content. Starts at 1 and increases by exactly one on every
   * successful draft update (curriculumMapNodeId, responseType, language, prompt); a rejected or
   * no-op update never moves it. Never caller-settable. A rubric version only counts towards
   * question verification while its questionRevision equals this value.
   */
  contentRevision: number;
  createdAt: string;
  updatedAt: string;
}

/**
 * One marking criterion. `descriptor` is original editorial wording, never a mark scheme.
 *
 * Core matches these key names EXACTLY and case-sensitively: `Id`, `Marks`, or `Descriptor` are
 * rejected as `invalid_rubric`, as are unknown keys and duplicate keys.
 */
export interface RubricCriterion {
  id: string;
  /** Positive integer; criterion marks must sum exactly to the version's maxMarks. */
  marks: number;
  descriptor?: string;
}

/**
 * Validation-safe rubric structure Core accepts. Keys are exact and response-aware; unknown keys,
 * case variants, duplicate keys, incomplete MCQ feedback, and feedback on non-MCQ are rejected.
 */
export interface RubricStructure {
  criteria: RubricCriterion[];
  /** Required for MCQ rubrics and prohibited for other response types. Editorial reads only. */
  answerKey?: RubricAnswerKey;
  /** Required for MCQ rubrics and prohibited for other response types. Editorial data only. */
  feedback?: RubricMCQFeedback;
}

export interface RubricAnswerKey {
  correctOptionId?: string;
  correctOptionIds?: string[];
  acceptedAnswers?: string[];
  numericValue?: number;
  tolerance?: number;
}

/**
 * An immutable, numbered rubric for a question. Its question, question revision, version,
 * structure, maximum marks, and creator never change after creation (enforced by a database
 * trigger); only verification metadata does.
 */
export interface QuestionRubricVersion {
  id: string;
  questionId: string;
  /** Positive, allocated server-side per question — never caller-supplied. */
  version: number;
  /**
   * The question's contentRevision when this version was created: the exact question content the
   * reviewer marked against. Stamped server-side. Editing the question makes older versions stale
   * for question verification — they stay verified, readable, and immutable, but a question can
   * only be verified with a verified version at its current revision.
   */
  questionRevision: number;
  rubric: RubricStructure;
  maxMarks: number;
  status: RubricVersionStatus;
  /** Verified Clerk subject that created the version. */
  createdBy: string;
  /** Verified Clerk subject that verified it; null while draft. */
  reviewedBy: string | null;
  createdAt: string;
  updatedAt: string;
}

/** Fields shared by every draft-question create. Status is never caller-settable. */
interface CreateQuestionRequestBase {
  syllabusId: string;
  curriculumMapNodeId: string;
  language: string;
  prompt: string;
}

/** Create shape enforces options only for MCQ at compile time; Core remains runtime authority. */
type CreateQuestionContent =
  | { responseType: "multiple_choice" | "multi_select"; options: MultipleChoiceOption[] }
  | { responseType: "exact_short_answer" | "numeric_answer" | "short_answer" | "structured_response" | "essay"; options?: never };

type CreateQuestionOrigin =
  | { originType: "original"; contentSourceId?: never; sourceLocator?: never }
  | {
      originType: "licensed_adaptation";
      contentSourceId: string;
      /** Metadata locator only, such as human-entered paper/session/question or chapter/page. */
      sourceLocator: string;
    };

export type CreateQuestionRequest = CreateQuestionRequestBase & CreateQuestionContent & CreateQuestionOrigin;

/**
 * Update a draft question (editor/reviewer/admin). Only a draft question may be updated. At
 * least one field must be supplied and every supplied field must be non-empty. `syllabusId` is
 * immutable — re-point `curriculumMapNodeId` instead, which re-validates the syllabus match.
 */
export interface UpdateQuestionRequest {
  curriculumMapNodeId?: string;
  responseType?: QuestionResponseType;
  language?: string;
  prompt?: string;
  options?: MultipleChoiceOption[];
}

/** Append a draft rubric version to a draft question. The version number is server-allocated. */
export interface CreateQuestionRubricVersionRequest {
  rubric: RubricStructure;
  maxMarks: number;
}

/** Exact body for question verification and historical canonical-rubric repair. */
export interface SelectCanonicalRubricRequest {
  rubricVersion: number;
}

export type QuestionEventType =
  | "question_created"
  | "question_updated"
  | "question_verified"
  | "question_retired"
  | "rubric_version_created"
  | "rubric_version_verified"
  | "canonical_rubric_selected";

/**
 * Immutable audit record of a question or rubric-version mutation. Records which fields changed
 * (names only) and who changed them — never field values, never a prompt, never rubric structure.
 */
export interface QuestionEvent {
  id: string;
  questionId: string;
  eventType: QuestionEventType;
  actorId: string;
  eventTime: string;
  changedFields: string[];
  createdAt: string;
}

// --- Learner-safe verified-question delivery (T-0015) ---
// The first surface any `learner`-role session may call. A strictly read-only, structurally
// reduced projection over the private editorial `Question`/`QuestionRubricVersion` types above:
// it can never carry `status`, `canonicalRubricVersionId`, a rubric, an answer key, marks, event
// data, actor identity, timestamps, origin/provenance, licence facts, or internal source metadata
// (source id/URL/hash). Mirrors
// services/core/internal/learner, which independently defines its own Go types (no import of the
// question package) so the projection cannot gain an editorial field by accident. `GET
// /learner/questions*` requires `learner_question:read`, held by every recognized role
// (learner/editor/reviewer/admin); unknown roles are denied. Core re-validates on every read that
// the question is verified, its canonical rubric exists/is verified/belongs to the question/
// matches the question's current content revision, its curriculum-map node is verified, and that
// node's content source is approved and still catalogue-linked to the question's syllabus — never
// cached, never a "latest rubric" fallback. `GET /learner/syllabuses` (review fix) is the same
// permission-gated surface for safe active-syllabus discovery, so the practice screen never asks
// a learner to type an opaque syllabus id. See docs/learner-question-delivery.md and D-0017.

/** Learner-visible MCQ option. Deliberately distinct from `MultipleChoiceOption` above — this
 * type can never gain a correctness flag; the answer key lives only in rubric data this
 * surface never reads. */
export interface LearnerQuestionOption {
  id: string;
  label: string;
}

/**
 * The explicit, exhaustive learner-safe question contract. This interface intentionally does
 * NOT extend `Question`: every field is listed here by hand so a future edit to the editorial
 * `Question` type (e.g. adding a sensitive field) can never widen what a learner receives.
 */
export interface LearnerQuestion {
  id: string;
  syllabusId: string;
  curriculumMapNodeId: string;
  responseType: QuestionResponseType;
  language: string;
  prompt: string;
  /** Present only for multiple_choice questions; null otherwise. */
  options: LearnerQuestionOption[] | null;
  contentRevision: number;
}

/**
 * The learner-safe active-syllabus discovery projection returned by `GET /learner/syllabuses`.
 * Deliberately distinct from the editorial `Syllabus` type above — written by hand, not derived
 * from it — so a future field added to the editorial catalogue record (rights data, timestamps,
 * internal status) can never widen what a learner-role caller receives. Only `active` catalogue
 * syllabuses are ever returned.
 */
export interface LearnerSyllabus {
  id: string;
  board: string;
  syllabusCode: SyllabusCode;
  qualification: string;
  /** Tier/route within a syllabus (e.g. "Extended"); null when not applicable. */
  track: string | null;
  displayName: string;
}

/**
 * Learner-safe module-picker entry returned by `GET /learner/modules`. A module appears only
 * while it contains at least one question that passes the live learner eligibility gate. This
 * is deliberately hand-written rather than based on editorial curriculum-map types: it cannot
 * inherit source, locator, provenance, lifecycle, parent, or audit fields.
 */
export interface LearnerModule {
  id: string;
  syllabusId: string;
  code: string;
  label: string;
}

// --- Private admin upload intake (T-0030) ---
// Metadata-only contracts. File bytes and AI output never travel through shared/learner types.
export type PrivateUploadStatus = "quarantined" | "scan_clean" | "review_pending" | "deletion_requested";
export interface PrivateUpload {
  id: string;
  objectRef: string;
  originalFilename: string;
  mediaType: "application/pdf";
  byteSize: number;
  sha256: string;
  contentSourceId: string | null;
  status: PrivateUploadStatus;
  retentionState: "retained" | "deletion_requested";
  uploadedBy: string;
  createdAt: string;
  updatedAt: string;
}
export interface QueuePrivateUploadReviewRequest { contentSourceId: string; }
export interface PrivateUploadReviewJob {
  id: string; uploadId: string; contentSourceId: string;
  status: "queued" | "completed" | "published" | "rejected";
  adapterVersion: "sonnet-review-v1"; createdAt: string;
}

export interface RubricIncorrectExplanation {
  optionId: string;
  explanation: string;
}

export interface RubricMCQFeedback {
  correctExplanation: string;
  incorrectExplanations: RubricIncorrectExplanation[];
}

// --- Learner-safe Practice Mode attempts (T-0016) ---
// Written independently from editorial Question/Rubric contracts. Open acknowledgements reveal
// no answer or rubric pin. Submitted results expose only deterministic canonical MCQ feedback.
export type LearnerAttemptStatus = "open";

export interface LearnerAttempt {
  attemptId: string;
  questionId: string;
  status: LearnerAttemptStatus;
  maxMarks: number;
  responseType?: QuestionResponseType;
}

export interface LearnerIncorrectExplanation {
  optionId: string;
  explanation: string;
}

export interface LearnerMCQFeedback {
  correctExplanation: string;
  incorrectExplanations: LearnerIncorrectExplanation[];
}

export interface LearnerAttemptResult {
  attemptId: string;
  questionId: string;
  responseType?: QuestionResponseType;
  answer?: LearnerAnswer;
  resultStatus?: "marked" | "pending_review";
  selectedOptionId: string;
  correctOptionId: string;
  isCorrect: boolean;
  awardedMarks: number;
  maxMarks: number;
  feedback: LearnerMCQFeedback;
}

/** Learner submission payload. Answer keys are never included in question projections. */
export type LearnerAnswer =
  | string
  | { selectedOptionId: string }
  | { selectedOptionIds: string[] }
  | { text: string }
  | { value: number };

/** Learner-safe AI written-marking lifecycle. It intentionally excludes the submitted answer,
 * rubric, canonical version, source/provenance and internal provider job trace. */
export type LearnerWrittenMarkingStatus = "pending" | "accepted" | "withheld";
export interface LearnerCriterionMark { criterionId: string; marksAwarded: number; feedback: string; }
export interface LearnerWrittenMarkingResult { criterionMarks: LearnerCriterionMark[]; awardedMarks: number; maxMarks: number; model: string; modelVersion: string; costUsdMicros: number; confidence: number; }
export interface LearnerWrittenMarking { attemptId: string; status: LearnerWrittenMarkingStatus; retryCount: number; withheldReason?: string; result?: LearnerWrittenMarkingResult; }

/** Owner-only aggregate derived from immutable completed-outcome snapshots. It deliberately
 * excludes answers, question IDs, rubrics, sources, provenance, model data and costs. */
export interface LearnerAnalyticsSummary {
  scoredItems: number; awardedMarks: number; possibleMarks: number;
  pendingMarking: number; withheldMarking: number;
  syllabuses: Array<{ syllabusId: string; scoredItems: number; awardedMarks: number; possibleMarks: number }>;
  modules: Array<{ syllabusId: string; moduleId: string; moduleCode: string; moduleLabel: string; scoredItems: number; awardedMarks: number; possibleMarks: number }>;
  recentActivity: Array<{ eventType: "deterministic_scored" | "pending_marking" | "automated_marking_accepted" | "automated_marking_withheld"; moduleLabel: string; awardedMarks?: number; maxMarks?: number; occurredAt: string }>;
}
