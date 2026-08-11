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

type Method = "GET" | "POST" | "PATCH";

export type LearnerOperation =
  | { kind: "listLearnerQuestions"; syllabusId: string; curriculumMapNodeId?: string }
  | { kind: "getLearnerQuestion"; id: string }
  | { kind: "listLearnerSyllabuses" }
	| { kind: "listLearnerModules"; syllabusId: string }
  | { kind: "createLearnerAttempt"; questionId: string }
  | { kind: "submitLearnerAttempt"; attemptId: string; answer: { selectedOptionId?: string; selectedOptionIds?: string[]; text?: string; value?: number } }
  | { kind: "submitLearnerAttempt"; attemptId: string; selectedOptionId: string }
  | { kind: "createAssessmentSession"; body: { mode: "practice" | "exam"; syllabusId: string; curriculumMapNodeId?: string; questionCount: number; durationSeconds?: number } }
  | { kind: "getAssessmentSession"; id: string }
  | { kind: "saveAssessmentResponse"; id: string; body: { ordinal: number; expectedVersion: number; answer: Record<string, unknown> } }
  | { kind: "submitAssessmentSession"; id: string }
  | { kind: "getAssessmentResult"; id: string }
  | { kind: "requestWrittenMarking"; attemptId: string }
  | { kind: "getWrittenMarking"; attemptId: string };

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

function requireValidAnswer(answer: { selectedOptionId?: string; selectedOptionIds?: string[]; text?: string; value?: number }): string {
  const keys = Object.keys(answer).filter((key) => (answer as Record<string, unknown>)[key] !== undefined);
  if (keys.length !== 1) throw new LearnerProxyError(400, "invalid_answer", "answer is invalid");
  if (typeof answer.selectedOptionId === "string") return JSON.stringify({ selectedOptionId: requireValidOptionId(answer.selectedOptionId) });
  if (Array.isArray(answer.selectedOptionIds) && answer.selectedOptionIds.length > 0 && answer.selectedOptionIds.length <= 6 && answer.selectedOptionIds.every((id) => typeof id === "string" && requireValidOptionId(id))) return JSON.stringify({ selectedOptionIds: answer.selectedOptionIds.map(requireValidOptionId) });
  if (typeof answer.text === "string" && answer.text.trim() && Array.from(answer.text).length <= 5000) return JSON.stringify({ text: answer.text.trim() });
  if (typeof answer.value === "number" && Number.isFinite(answer.value)) return JSON.stringify({ value: answer.value });
  throw new LearnerProxyError(400, "invalid_answer", "answer is invalid");
}

function requireValidOptionId(id: string): string {
  const normalized = id.trim();
  if (!normalized || Array.from(normalized).length > 64) {
    throw new LearnerProxyError(400, "invalid_option", "selected option is invalid");
  }
  return normalized;
}

function requireSessionCreate(body: { mode: "practice" | "exam"; syllabusId: string; curriculumMapNodeId?: string; questionCount: number; durationSeconds?: number }): string {
  if ((body.mode !== "practice" && body.mode !== "exam") || !Number.isSafeInteger(body.questionCount) || body.questionCount < 1 || body.questionCount > 500) throw new LearnerProxyError(400, "invalid_input", "assessment setup is invalid");
  const safe: Record<string, unknown> = { mode: body.mode, syllabusId: requireValidId(body.syllabusId), questionCount: body.questionCount };
  if (body.curriculumMapNodeId !== undefined) safe.curriculumMapNodeId = requireValidId(body.curriculumMapNodeId);
  if (body.mode === "exam") { if (!Number.isSafeInteger(body.durationSeconds) || body.durationSeconds! < 60 || body.durationSeconds! > 14400) throw new LearnerProxyError(400, "invalid_input", "assessment setup is invalid"); safe.durationSeconds = body.durationSeconds; }
  return JSON.stringify(safe);
}
function requireSaveResponse(body: { ordinal: number; expectedVersion: number; answer: Record<string, unknown> }): string {
  if (!Number.isSafeInteger(body.ordinal) || body.ordinal < 1 || !Number.isSafeInteger(body.expectedVersion) || body.expectedVersion < 0) throw new LearnerProxyError(400, "invalid_input", "assessment response is invalid");
  const answer = JSON.parse(requireValidAnswer(body.answer as { selectedOptionId?: string; selectedOptionIds?: string[]; text?: string; value?: number }));
  return JSON.stringify({ ordinal: body.ordinal, expectedVersion: body.expectedVersion, answer });
}

function resolveRoute(op: LearnerOperation): { method: Method; path: string; body?: string } {
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
    case "listLearnerSyllabuses":
      return { method: "GET", path: "/learner/syllabuses" };
	case "listLearnerModules":
		return { method: "GET", path: `/learner/modules?${new URLSearchParams({ syllabusId: requireValidId(op.syllabusId) }).toString()}` };
    case "createLearnerAttempt":
      return { method: "POST", path: `/learner/questions/${requireValidId(op.questionId)}/attempts` };
    case "submitLearnerAttempt":
      return {
        method: "POST",
        path: `/learner/attempts/${requireValidId(op.attemptId)}/submit`,
        body: requireValidAnswer("answer" in op ? op.answer : { selectedOptionId: op.selectedOptionId }),
      };
		case "createAssessmentSession":
			return { method: "POST", path: "/learner/assessment-sessions", body: requireSessionCreate(op.body) };
		case "getAssessmentSession":
			return { method: "GET", path: `/learner/assessment-sessions/${requireValidId(op.id)}` };
		case "saveAssessmentResponse":
			return { method: "PATCH", path: `/learner/assessment-sessions/${requireValidId(op.id)}/responses`, body: requireSaveResponse(op.body) };
		case "submitAssessmentSession":
			return { method: "POST", path: `/learner/assessment-sessions/${requireValidId(op.id)}/submit` };
		case "getAssessmentResult":
			return { method: "GET", path: `/learner/assessment-sessions/${requireValidId(op.id)}/result` };
		case "requestWrittenMarking":
			return { method: "POST", path: `/learner/attempts/${requireValidId(op.attemptId)}/marking` };
		case "getWrittenMarking":
			return { method: "GET", path: `/learner/attempts/${requireValidId(op.attemptId)}/marking` };
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
        ...(route.body === undefined ? {} : { "Content-Type": "application/json" }),
      },
      body: route.body,
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
