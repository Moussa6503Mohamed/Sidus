import "server-only";
import { auth } from "@clerk/nextjs/server";

// The learner-facing mirror of lib/editorial/core-proxy.ts's "no open proxy" boundary (D-0011),
// kept as an intentionally separate module and operation union rather than extending
// EditorialOperation: the two surfaces have different audiences (every recognized role vs.
// editorial-only) and different Core routes (T-0015's GET /learner/questions* vs. T-0007's
// GET/POST/PATCH /questions*), so keeping them apart means a change to one union can never widen
// the other by accident. A LearnerOperation is a closed union mapped to exactly one fixed Core
// GET path template — there is no caller-controlled target URL, and no write operation exists
// here at all.

const REQUEST_TIMEOUT_MS = 10_000;

// UUID-shaped ids only. Rejects path-traversal/segment-injection attempts before any URL is
// built, independent of Core's own id validation — defense in depth at the boundary that
// actually assembles the URL.
const ID_PATTERN = /^[A-Za-z0-9_-]{1,128}$/;

export class LearnerProxyError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

type Method = "GET";

export type LearnerOperation =
  | { kind: "listLearnerQuestions"; syllabusId: string; curriculumMapNodeId?: string }
  | { kind: "getLearnerQuestion"; id: string };

export interface LearnerProxySuccess {
  status: number;
  body: unknown;
}

function requireValidId(id: string): string {
  if (!ID_PATTERN.test(id)) {
    throw new LearnerProxyError(400, "invalid_id", "identifier is not a valid resource id");
  }
  return id;
}

function resolveRoute(op: LearnerOperation): { method: Method; path: string } {
  switch (op.kind) {
    case "listLearnerQuestions": {
      const query = new URLSearchParams({ syllabusId: requireValidId(op.syllabusId) });
      if (op.curriculumMapNodeId !== undefined) {
        query.set("curriculumMapNodeId", requireValidId(op.curriculumMapNodeId));
      }
      return { method: "GET", path: `/learner/questions?${query.toString()}` };
    }
    case "getLearnerQuestion":
      return { method: "GET", path: `/learner/questions/${requireValidId(op.id)}` };
  }
}

/**
 * Calls exactly one allowlisted Core learner-delivery endpoint for op. Fails closed, in order:
 * unconfigured Core base URL (503) before touching auth, then ids/query values are validated by
 * resolveRoute before any Clerk lookup, then a missing/empty session token (401) before any
 * network call. Mirrors callCore's Core-5xx sanitization and redirect refusal exactly.
 */
export async function callCoreLearner(op: LearnerOperation): Promise<LearnerProxySuccess> {
  const baseUrl = process.env.SIDUS_CORE_API_URL;
  if (!baseUrl) {
    throw new LearnerProxyError(503, "service_unavailable", "the practice service is not configured");
  }

  // Resolve and validate caller-controlled ids/filters before Clerk or Core network work.
  const route = resolveRoute(op);

  const { getToken } = await auth();
  const token = await getToken();
  if (!token) {
    throw new LearnerProxyError(401, "unauthorized", "sign-in is required");
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
      },
      redirect: "error",
      signal: controller.signal,
    });
  } catch {
    // Covers network failure and a Core redirect (redirect: "error" makes fetch throw instead
    // of following it or forwarding the bearer token to the redirect target).
    throw new LearnerProxyError(502, "upstream_unavailable", "the practice service is temporarily unavailable");
  } finally {
    clearTimeout(timeout);
  }

  // Never forward a Core 5xx body: it may contain raw Go/database/driver error text. Consume it
  // (so the connection is released) but discard it — do not log it, the target URL, or the token.
  if (response.status >= 500) {
    await response.text().catch(() => undefined);
    throw new LearnerProxyError(502, "upstream_error", "the practice service is temporarily unavailable");
  }

  const text = await response.text();
  if (!text) {
    return { status: response.status, body: null };
  }
  try {
    return { status: response.status, body: JSON.parse(text) };
  } catch {
    throw new LearnerProxyError(502, "upstream_error", "the practice service returned an unexpected response");
  }
}
