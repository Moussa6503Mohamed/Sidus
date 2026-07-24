export type SyllabusCode = "0610" | "5090";

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
