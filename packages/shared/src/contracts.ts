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
