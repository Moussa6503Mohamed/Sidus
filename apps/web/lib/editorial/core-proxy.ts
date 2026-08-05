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
  | { kind: "listSyllabuses" };

export interface ProxySuccess {
  status: number;
  body: unknown;
}

function requireValidId(id: string): string {
  if (!ID_PATTERN.test(id)) {
    throw new ProxyError(400, "invalid_id", "identifier is not a valid resource id");
  }
  return id;
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

  const { getToken } = await auth();
  const token = await getToken();
  if (!token) {
    throw new ProxyError(401, "unauthorized", "sign-in is required");
  }

  const route = resolveRoute(op);
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
