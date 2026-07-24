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

export interface ApproveContentSourceRequest {
  reviewerId: string;
  decisionDate?: string;
}

export interface RejectContentSourceRequest {
  reviewerId: string;
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
 * Update a pending content source. `actorId` is required. At least one updatable field
 * must be supplied; supplied fields must be non-empty. Only pending sources may be updated.
 */
export interface UpdateContentSourceRequest {
  actorId: string;
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
