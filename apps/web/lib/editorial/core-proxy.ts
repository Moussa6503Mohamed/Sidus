import "server-only";
import { auth } from "@clerk/nextjs/server";

// Every editorial BFF route handler calls into this module and nowhere else touches
// SIDUS_CORE_API_URL or forwards a bearer token. There is no caller-controlled target URL: an
// Operation is a closed union, and resolveRoute maps each variant to a fixed Core path
// template with only a validated {id} interpolated in. This is what "no open proxy" means in
// practice — a caller can select *which* allowlisted operation to invoke, never *where* it goes.

const REQUEST_TIMEOUT_MS = 10_000;
const MAX_REQUEST_BODY_BYTES = 100_000;

// UUID-shaped ids only. Rejects path-traversal/segment-injection attempts (e.g. "../x", an
// encoded slash decoded to "a/b") before any URL is built, independent of Core's own id
// validation (D-0010) — defense in depth at the boundary that actually assembles the URL.
const ID_PATTERN = /^[A-Za-z0-9_-]{1,128}$/;
const CURRICULUM_MAP_STATUSES = new Set(["draft", "verified", "retired", "all"]);
const QUESTION_STATUSES = new Set(["draft", "verified", "retired", "all"]);
const POSITIVE_VERSION_PATTERN = /^[1-9][0-9]{0,9}$/;
const MAX_POSTGRES_INTEGER = 2_147_483_647;

export class ProxyError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

type Method = "GET" | "POST" | "PATCH";

export type EditorialOperation =
  | { kind: "listContentSources"; status?: string }
  | { kind: "createContentSource" }
  | { kind: "getContentSource"; id: string }
  | { kind: "updateContentSource"; id: string }
  | { kind: "approveContentSource"; id: string }
  | { kind: "rejectContentSource"; id: string }
  | { kind: "listSyllabuses" }
  | { kind: "listCurriculumMapNodes"; syllabusId: string; status?: string }
  | { kind: "getCurriculumMapNode"; id: string }
  | { kind: "createCurriculumMapNode" }
  | { kind: "updateCurriculumMapNode"; id: string }
  | { kind: "verifyCurriculumMapNode"; id: string }
  | { kind: "retireCurriculumMapNode"; id: string }
  | { kind: "listQuestions"; syllabusId: string; curriculumMapNodeId?: string; status?: string }
  | { kind: "getQuestion"; id: string }
  | { kind: "createQuestion" }
  | { kind: "updateQuestion"; id: string }
  | { kind: "verifyQuestion"; id: string }
  | { kind: "setQuestionCanonicalRubric"; id: string }
  | { kind: "retireQuestion"; id: string }
  | { kind: "listQuestionRubricVersions"; id: string }
  | { kind: "createQuestionRubricVersion"; id: string }
  | { kind: "verifyQuestionRubricVersion"; id: string; version: string };

export type UploadOperation =
  | { kind: "listPrivateUploads" }
  | { kind: "createPrivateUpload"; filename: string }
  | { kind: "markUploadScanClean"; id: string }
  | { kind: "queueUploadReview"; id: string }
  | { kind: "requestUploadDeletion"; id: string };

export interface ProxySuccess {
  status: number;
  body: unknown;
}

function resolveUploadRoute(op: UploadOperation): { method: Method; path: string } {
  switch (op.kind) {
    case "listPrivateUploads": return { method: "GET", path: "/private-uploads" };
    case "createPrivateUpload": return { method: "POST", path: "/private-uploads" };
    case "markUploadScanClean": return { method: "POST", path: `/private-uploads/${requireValidId(op.id)}/scan-clean` };
    case "queueUploadReview": return { method: "POST", path: `/private-uploads/${requireValidId(op.id)}/review-jobs` };
    case "requestUploadDeletion": return { method: "POST", path: `/private-uploads/${requireValidId(op.id)}/deletion-request` };
  }
}

/** Fixed-route binary upload proxy. PDF bytes move only admin browser -> BFF -> Core private
 * quarantine. It never logs, parses, caches, or returns bytes. */
export async function callUploadCore(op: UploadOperation, body?: ArrayBuffer | string): Promise<ProxySuccess> {
  const baseUrl = process.env.SIDUS_CORE_API_URL;
  if (!baseUrl) throw new ProxyError(503, "service_unavailable", "the editorial service is not configured");
  const route = resolveUploadRoute(op);
  const { getToken } = await auth(); const token = await getToken();
  if (!token) throw new ProxyError(401, "unauthorized", "sign-in is required");
  const controller = new AbortController(); const timeout = setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS);
  try {
    const response = await fetch(`${baseUrl.replace(/\/+$/, "")}${route.path}`, {
      method: route.method, redirect: "error", signal: controller.signal,
      headers: { Authorization: `Bearer ${token}`, Accept: "application/json", ...(op.kind === "createPrivateUpload" ? { "Content-Type": "application/pdf", "X-Sidus-Upload-Filename": op.filename } : body !== undefined ? { "Content-Type": "application/json" } : {}) },
      body,
    });
    if (response.status >= 500) { await response.text().catch(() => undefined); throw new ProxyError(502, "upstream_error", "the editorial service is temporarily unavailable"); }
    const text = await response.text(); return { status: response.status, body: text ? JSON.parse(text) : null };
  } catch (err) { if (err instanceof ProxyError) throw err; throw new ProxyError(502, "upstream_unavailable", "the editorial service is temporarily unavailable"); }
  finally { clearTimeout(timeout); }
}

function requireValidId(id: string): string {
  if (!ID_PATTERN.test(id)) {
    throw new ProxyError(400, "invalid_id", "identifier is not a valid resource id");
  }
  return id;
}

function requireValidCurriculumMapStatus(status: string): string {
  if (!CURRICULUM_MAP_STATUSES.has(status)) {
    throw new ProxyError(400, "invalid_status", "status must be one of: draft, verified, retired, all");
  }
  return status;
}

function requireValidQuestionStatus(status: string): string {
  if (!QUESTION_STATUSES.has(status)) {
    throw new ProxyError(400, "invalid_status", "status must be one of: draft, verified, retired, all");
  }
  return status;
}

function requireValidVersion(version: string): string {
  if (!POSITIVE_VERSION_PATTERN.test(version) || Number(version) > MAX_POSTGRES_INTEGER) {
    throw new ProxyError(400, "invalid_version", "version must be a positive integer");
  }
  return version;
}

function resolveRoute(op: EditorialOperation): { method: Method; path: string } {
  switch (op.kind) {
    case "listContentSources": {
      const query = op.status ? `?status=${encodeURIComponent(op.status)}` : "";
      return { method: "GET", path: `/content-sources${query}` };
    }
    case "createContentSource":
      return { method: "POST", path: "/content-sources" };
    case "getContentSource":
      return { method: "GET", path: `/content-sources/${requireValidId(op.id)}` };
    case "updateContentSource":
      return { method: "PATCH", path: `/content-sources/${requireValidId(op.id)}` };
    case "approveContentSource":
      return { method: "POST", path: `/content-sources/${requireValidId(op.id)}/approve` };
    case "rejectContentSource":
      return { method: "POST", path: `/content-sources/${requireValidId(op.id)}/reject` };
    case "listSyllabuses":
      return { method: "GET", path: "/catalogue/syllabuses" };
    case "listCurriculumMapNodes": {
      const query = new URLSearchParams({ syllabusId: requireValidId(op.syllabusId) });
      if (op.status !== undefined) query.set("status", requireValidCurriculumMapStatus(op.status));
      return { method: "GET", path: `/curriculum-map/nodes?${query.toString()}` };
    }
    case "getCurriculumMapNode":
      return { method: "GET", path: `/curriculum-map/nodes/${requireValidId(op.id)}` };
    case "createCurriculumMapNode":
      return { method: "POST", path: "/curriculum-map/nodes" };
    case "updateCurriculumMapNode":
      return { method: "PATCH", path: `/curriculum-map/nodes/${requireValidId(op.id)}` };
    case "verifyCurriculumMapNode":
      return { method: "POST", path: `/curriculum-map/nodes/${requireValidId(op.id)}/verify` };
    case "retireCurriculumMapNode":
      return { method: "POST", path: `/curriculum-map/nodes/${requireValidId(op.id)}/retire` };
    case "listQuestions": {
      const query = new URLSearchParams({ syllabusId: requireValidId(op.syllabusId) });
      if (op.curriculumMapNodeId !== undefined) {
        query.set("curriculumMapNodeId", requireValidId(op.curriculumMapNodeId));
      }
      if (op.status !== undefined) query.set("status", requireValidQuestionStatus(op.status));
      return { method: "GET", path: `/questions?${query.toString()}` };
    }
    case "getQuestion":
      return { method: "GET", path: `/questions/${requireValidId(op.id)}` };
    case "createQuestion":
      return { method: "POST", path: "/questions" };
    case "updateQuestion":
      return { method: "PATCH", path: `/questions/${requireValidId(op.id)}` };
    case "verifyQuestion":
      return { method: "POST", path: `/questions/${requireValidId(op.id)}/verify` };
    case "setQuestionCanonicalRubric":
      return { method: "POST", path: `/questions/${requireValidId(op.id)}/canonical-rubric` };
    case "retireQuestion":
      return { method: "POST", path: `/questions/${requireValidId(op.id)}/retire` };
    case "listQuestionRubricVersions":
      return { method: "GET", path: `/questions/${requireValidId(op.id)}/rubric-versions` };
    case "createQuestionRubricVersion":
      return { method: "POST", path: `/questions/${requireValidId(op.id)}/rubric-versions` };
    case "verifyQuestionRubricVersion":
      return {
        method: "POST",
        path: `/questions/${requireValidId(op.id)}/rubric-versions/${requireValidVersion(op.version)}/verify`,
      };
  }
}

/**
 * Calls exactly one allowlisted Core endpoint for op. Fails closed, in order: unconfigured
 * Core base URL (503) before touching auth, then missing/empty session token (401) before any
 * network call — so a misconfigured deployment or a signed-out caller never reaches Core, and
 * never via an error message that reveals which one.
 */
export async function callCore(op: EditorialOperation, rawBody?: string): Promise<ProxySuccess> {
  const baseUrl = process.env.SIDUS_CORE_API_URL;
  if (!baseUrl) {
    throw new ProxyError(503, "service_unavailable", "the editorial service is not configured");
  }

  // Resolve and validate caller-controlled ids/filters before Clerk or Core network work.
  const route = resolveRoute(op);

  const { getToken } = await auth();
  const token = await getToken();
  if (!token) {
    throw new ProxyError(401, "unauthorized", "sign-in is required");
  }

  const target = `${baseUrl.replace(/\/+$/, "")}${route.path}`;

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS);
  let response: Response;
  try {
    response = await fetch(target, {
      method: route.method,
      headers: {
        Authorization: `Bearer ${token}`,
        Accept: "application/json",
        ...(rawBody !== undefined ? { "Content-Type": "application/json" } : {}),
      },
      body: rawBody,
      redirect: "error",
      signal: controller.signal,
    });
  } catch {
    // Covers network failure and a Core redirect (redirect: "error" makes fetch throw instead
    // of following it or forwarding the bearer token to the redirect target).
    throw new ProxyError(502, "upstream_unavailable", "the editorial service is temporarily unavailable");
  } finally {
    clearTimeout(timeout);
  }

  // Never forward a Core 5xx body: it may contain raw Go/database/driver error text. Consume it
  // (so the connection is released) but discard it — do not log it, the target URL, or the token.
  if (response.status >= 500) {
    await response.text().catch(() => undefined);
    throw new ProxyError(502, "upstream_error", "the editorial service is temporarily unavailable");
  }

  const text = await response.text();
  if (!text) {
    return { status: response.status, body: null };
  }
  try {
    return { status: response.status, body: JSON.parse(text) };
  } catch {
    throw new ProxyError(502, "upstream_error", "the editorial service returned an unexpected response");
  }
}

/**
 * Reads and minimally validates an incoming request body before it is forwarded verbatim to
 * Core: must be declared JSON, within a hard size limit, and syntactically valid JSON. The raw
 * text is returned unmodified (never re-serialized) so Core's own strict decoding — the real
 * authority on shape — sees exactly what the caller sent.
 */
export async function readSafeJsonBody(request: Request): Promise<string> {
  const contentType = request.headers.get("content-type") ?? "";
  if (!contentType.toLowerCase().includes("application/json")) {
    throw new ProxyError(400, "invalid_content_type", "request body must be application/json");
  }

  const text = await request.text();
  if (text.length > MAX_REQUEST_BODY_BYTES) {
    throw new ProxyError(413, "payload_too_large", "request body is too large");
  }
  try {
    JSON.parse(text);
  } catch {
    throw new ProxyError(400, "invalid_json", "request body must be valid JSON");
  }
  return text;
}

/** Validates fixed canonical-selection shape at BFF boundary, then preserves raw JSON for Core's
 * stricter duplicate-key/token validation. */
export async function readCanonicalRubricBody(request: Request): Promise<string> {
  const text = await readSafeJsonBody(request);
  const value = JSON.parse(text) as unknown;
  if (value === null || Array.isArray(value) || typeof value !== "object") {
    throw new ProxyError(400, "invalid_json", "request body must contain only a positive rubricVersion");
  }
  const keys = Object.keys(value);
  const version = (value as Record<string, unknown>).rubricVersion;
  if (
    keys.length !== 1 || keys[0] !== "rubricVersion" ||
    !Number.isInteger(version) || (version as number) <= 0 || (version as number) > MAX_POSTGRES_INTEGER
  ) {
    throw new ProxyError(400, "invalid_rubric_version", "request body must contain only a positive rubricVersion");
  }
  return text;
}
