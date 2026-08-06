// @vitest-environment node
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { auth } from "@clerk/nextjs/server";
import { callCoreLearner, LearnerProxyError } from "./core-proxy";

vi.mock("@clerk/nextjs/server", () => ({
  auth: vi.fn(),
}));

const mockedAuth = vi.mocked(auth);

function mockSignedIn(token: string | null = "test-token") {
  mockedAuth.mockResolvedValue({
    getToken: vi.fn().mockResolvedValue(token),
  } as never);
}

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json" },
  });
}

describe("callCoreLearner", () => {
  const originalFetch = global.fetch;
  let fetchSpy: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    vi.stubEnv("SIDUS_CORE_API_URL", "http://core.internal:8080");
    fetchSpy = vi.fn();
    global.fetch = fetchSpy as unknown as typeof fetch;
  });

  afterEach(() => {
    vi.unstubAllEnvs();
    vi.clearAllMocks();
    global.fetch = originalFetch;
  });

  it("fails closed with 503 when SIDUS_CORE_API_URL is missing, without calling auth or fetch", async () => {
    vi.unstubAllEnvs();
    mockSignedIn();

    const err = await callCoreLearner({ kind: "listLearnerQuestions", syllabusId: "syl-1" }).catch((e) => e);

    expect(err).toBeInstanceOf(LearnerProxyError);
    expect((err as LearnerProxyError).status).toBe(503);
    expect(mockedAuth).not.toHaveBeenCalled();
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it("fails closed with 401 when the session token is missing, without calling fetch", async () => {
    mockSignedIn(null);

    const err = await callCoreLearner({ kind: "listLearnerQuestions", syllabusId: "syl-1" }).catch((e) => e);

    expect(err).toBeInstanceOf(LearnerProxyError);
    expect((err as LearnerProxyError).status).toBe(401);
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it.each([
    ["empty", ""],
    ["overlong", "a".repeat(129)],
    ["path-traversal", "../catalogue/syllabuses"],
    ["slash", "abc/def"],
  ])("rejects %s syllabusId before auth or fetch", async (_name, syllabusId) => {
    mockSignedIn();

    const err = await callCoreLearner({ kind: "listLearnerQuestions", syllabusId }).catch((e) => e);

    expect(err).toBeInstanceOf(LearnerProxyError);
    expect((err as LearnerProxyError).status).toBe(400);
    expect((err as LearnerProxyError).code).toBe("invalid_id");
    expect(mockedAuth).not.toHaveBeenCalled();
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it("rejects an invalid curriculumMapNodeId filter before auth or fetch", async () => {
    mockSignedIn();

    const err = await callCoreLearner({
      kind: "listLearnerQuestions",
      syllabusId: "syl-1",
      curriculumMapNodeId: "../nodes",
    }).catch((e) => e);

    expect(err).toBeInstanceOf(LearnerProxyError);
    expect((err as LearnerProxyError).status).toBe(400);
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it("rejects an invalid getLearnerQuestion id before auth or fetch", async () => {
    mockSignedIn();

    const err = await callCoreLearner({ kind: "getLearnerQuestion", id: "question/1" }).catch((e) => e);

    expect(err).toBeInstanceOf(LearnerProxyError);
    expect((err as LearnerProxyError).status).toBe(400);
    expect(mockedAuth).not.toHaveBeenCalled();
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it("calls the exact fixed learner list path with the bearer token forwarded", async () => {
    mockSignedIn("abc.def.ghi");
    fetchSpy.mockResolvedValue(jsonResponse(200, { items: [] }));

    const result = await callCoreLearner({
      kind: "listLearnerQuestions",
      syllabusId: "syl-1",
      curriculumMapNodeId: "node-1",
    });

    expect(result).toEqual({ status: 200, body: { items: [] } });
    expect(fetchSpy).toHaveBeenCalledTimes(1);
    const [target, init] = fetchSpy.mock.calls[0] as [string, RequestInit];
    expect(target).toBe("http://core.internal:8080/learner/questions?syllabusId=syl-1&curriculumMapNodeId=node-1");
    expect(init.method).toBe("GET");
    expect(init.redirect).toBe("error");
    expect((init.headers as Record<string, string>).Authorization).toBe("Bearer abc.def.ghi");
  });

  it("calls the exact fixed learner get path", async () => {
    mockSignedIn();
    fetchSpy.mockResolvedValue(jsonResponse(200, { id: "q-1" }));

    await callCoreLearner({ kind: "getLearnerQuestion", id: "q-1" });

    const [target] = fetchSpy.mock.calls[0] as [string, RequestInit];
    expect(target).toBe("http://core.internal:8080/learner/questions/q-1");
  });

  it("never follows a Core redirect and never forwards the token to it", async () => {
    mockSignedIn();
    fetchSpy.mockImplementation(() => {
      throw new TypeError("Failed to fetch"); // redirect: "error" makes fetch throw
    });

    const err = await callCoreLearner({ kind: "listLearnerQuestions", syllabusId: "syl-1" }).catch((e) => e);

    expect(err).toBeInstanceOf(LearnerProxyError);
    expect((err as LearnerProxyError).status).toBe(502);
  });

  it("sanitizes a Core 5xx body instead of forwarding it", async () => {
    mockSignedIn();
    fetchSpy.mockResolvedValue(
      new Response("panic: raw driver text with a secret", { status: 500 }),
    );

    const err = await callCoreLearner({ kind: "listLearnerQuestions", syllabusId: "syl-1" }).catch((e) => e);

    expect(err).toBeInstanceOf(LearnerProxyError);
    expect((err as LearnerProxyError).status).toBe(502);
    expect((err as LearnerProxyError).code).toBe("upstream_error");
    expect((err as LearnerProxyError).message).not.toMatch(/secret|panic/);
  });

  it("calls the exact fixed learner syllabus discovery path with no query params", async () => {
    mockSignedIn("abc.def.ghi");
    fetchSpy.mockResolvedValue(jsonResponse(200, { items: [] }));

    const result = await callCoreLearner({ kind: "listLearnerSyllabuses" });

    expect(result).toEqual({ status: 200, body: { items: [] } });
    expect(fetchSpy).toHaveBeenCalledTimes(1);
    const [target, init] = fetchSpy.mock.calls[0] as [string, RequestInit];
    expect(target).toBe("http://core.internal:8080/learner/syllabuses");
    expect(init.method).toBe("GET");
    expect(init.redirect).toBe("error");
    expect((init.headers as Record<string, string>).Authorization).toBe("Bearer abc.def.ghi");
  });

  it("fails closed with 503 for listLearnerSyllabuses when SIDUS_CORE_API_URL is missing", async () => {
    vi.unstubAllEnvs();
    mockSignedIn();

    const err = await callCoreLearner({ kind: "listLearnerSyllabuses" }).catch((e) => e);

    expect(err).toBeInstanceOf(LearnerProxyError);
    expect((err as LearnerProxyError).status).toBe(503);
    expect(mockedAuth).not.toHaveBeenCalled();
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it("fails closed with 401 for listLearnerSyllabuses when the session token is missing", async () => {
    mockSignedIn(null);

    const err = await callCoreLearner({ kind: "listLearnerSyllabuses" }).catch((e) => e);

    expect(err).toBeInstanceOf(LearnerProxyError);
    expect((err as LearnerProxyError).status).toBe(401);
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it("passes through a Core 404 unchanged", async () => {
    mockSignedIn();
    fetchSpy.mockResolvedValue(jsonResponse(404, { error: "not_found", message: "question not found" }));

    const result = await callCoreLearner({ kind: "getLearnerQuestion", id: "missing" });

    expect(result).toEqual({ status: 404, body: { error: "not_found", message: "question not found" } });
  });
});
