// @vitest-environment node
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { auth } from "@clerk/nextjs/server";
import { callCore, ProxyError, readSafeJsonBody } from "./core-proxy";

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

describe("callCore", () => {
  const originalFetch = global.fetch;
  let fetchSpy: ReturnType<typeof vi.fn>;
  let logSpy: ReturnType<typeof vi.spyOn>;
  let errorSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    vi.stubEnv("SIDUS_CORE_API_URL", "http://core.internal:8080");
    fetchSpy = vi.fn();
    global.fetch = fetchSpy as unknown as typeof fetch;
    logSpy = vi.spyOn(console, "log").mockImplementation(() => {});
    errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
  });

  afterEach(() => {
    vi.unstubAllEnvs();
    vi.clearAllMocks();
    global.fetch = originalFetch;
    logSpy.mockRestore();
    errorSpy.mockRestore();
  });

  it("fails closed with 503 when SIDUS_CORE_API_URL is missing, without calling auth or fetch", async () => {
    vi.unstubAllEnvs();
    mockSignedIn();

    const err = await callCore({ kind: "listSyllabuses" }).catch((e) => e);

    expect(err).toBeInstanceOf(ProxyError);
    expect((err as ProxyError).status).toBe(503);
    expect(mockedAuth).not.toHaveBeenCalled();
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it("fails closed with 401 when the session token is missing, without calling fetch", async () => {
    mockSignedIn(null);

    const err = await callCore({ kind: "listSyllabuses" }).catch((e) => e);

    expect(err).toBeInstanceOf(ProxyError);
    expect((err as ProxyError).status).toBe(401);
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it("rejects a path-traversal id before building any URL, without calling fetch", async () => {
    mockSignedIn();

    const err = await callCore({ kind: "getContentSource", id: "../../catalogue/syllabuses" }).catch(
      (e) => e,
    );

    expect(err).toBeInstanceOf(ProxyError);
    expect((err as ProxyError).status).toBe(400);
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it("rejects an id containing a slash, without calling fetch", async () => {
    mockSignedIn();

    const err = await callCore({ kind: "updateContentSource", id: "abc/def" }).catch((e) => e);

    expect(err).toBeInstanceOf(ProxyError);
    expect((err as ProxyError).status).toBe(400);
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it("rejects a path-traversal curriculum-map node id before building any URL", async () => {
    mockSignedIn();

    const err = await callCore({ kind: "getCurriculumMapNode", id: "../nodes" }).catch((e) => e);

    expect(err).toBeInstanceOf(ProxyError);
    expect((err as ProxyError).status).toBe(400);
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it.each([
    ["empty", ""],
    ["overlong", "a".repeat(129)],
    ["unsafe", "../catalogue/syllabuses"],
  ])("rejects %s curriculum-map syllabusId before auth or fetch", async (_name, syllabusId) => {
    mockSignedIn();

    const err = await callCore({ kind: "listCurriculumMapNodes", syllabusId }).catch((e) => e);

    expect(err).toBeInstanceOf(ProxyError);
    expect((err as ProxyError).status).toBe(400);
    expect((err as ProxyError).code).toBe("invalid_id");
    expect(mockedAuth).not.toHaveBeenCalled();
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it.each(["", "bogus", "Draft"])(
    "rejects invalid curriculum-map status %j before auth or fetch",
    async (status) => {
      mockSignedIn();

      const err = await callCore({
        kind: "listCurriculumMapNodes",
        syllabusId: "syl-1",
        status,
      }).catch((e) => e);

      expect(err).toBeInstanceOf(ProxyError);
      expect((err as ProxyError).status).toBe(400);
      expect((err as ProxyError).code).toBe("invalid_status");
      expect(mockedAuth).not.toHaveBeenCalled();
      expect(fetchSpy).not.toHaveBeenCalled();
    },
  );

  const cases: Array<{
    name: string;
    op: Parameters<typeof callCore>[0];
    method: string;
    path: string;
  }> = [
    { name: "list", op: { kind: "listContentSources" }, method: "GET", path: "/content-sources" },
    {
      name: "list filtered",
      op: { kind: "listContentSources", status: "pending" },
      method: "GET",
      path: "/content-sources?status=pending",
    },
    { name: "create", op: { kind: "createContentSource" }, method: "POST", path: "/content-sources" },
    {
      name: "get",
      op: { kind: "getContentSource", id: "abc-123" },
      method: "GET",
      path: "/content-sources/abc-123",
    },
    {
      name: "update",
      op: { kind: "updateContentSource", id: "abc-123" },
      method: "PATCH",
      path: "/content-sources/abc-123",
    },
    {
      name: "approve",
      op: { kind: "approveContentSource", id: "abc-123" },
      method: "POST",
      path: "/content-sources/abc-123/approve",
    },
    {
      name: "reject",
      op: { kind: "rejectContentSource", id: "abc-123" },
      method: "POST",
      path: "/content-sources/abc-123/reject",
    },
    { name: "syllabuses", op: { kind: "listSyllabuses" }, method: "GET", path: "/catalogue/syllabuses" },
    {
      name: "curriculum map list",
      op: { kind: "listCurriculumMapNodes", syllabusId: "syl-1" },
      method: "GET",
      path: "/curriculum-map/nodes?syllabusId=syl-1",
    },
    {
      name: "curriculum map list filtered",
      op: { kind: "listCurriculumMapNodes", syllabusId: "syl-1", status: "draft" },
      method: "GET",
      path: "/curriculum-map/nodes?syllabusId=syl-1&status=draft",
    },
    {
      name: "curriculum map list all statuses",
      op: { kind: "listCurriculumMapNodes", syllabusId: "syl-1", status: "all" },
      method: "GET",
      path: "/curriculum-map/nodes?syllabusId=syl-1&status=all",
    },
    {
      name: "curriculum map get",
      op: { kind: "getCurriculumMapNode", id: "node-1" },
      method: "GET",
      path: "/curriculum-map/nodes/node-1",
    },
    {
      name: "curriculum map create",
      op: { kind: "createCurriculumMapNode" },
      method: "POST",
      path: "/curriculum-map/nodes",
    },
    {
      name: "curriculum map update",
      op: { kind: "updateCurriculumMapNode", id: "node-1" },
      method: "PATCH",
      path: "/curriculum-map/nodes/node-1",
    },
    {
      name: "curriculum map verify",
      op: { kind: "verifyCurriculumMapNode", id: "node-1" },
      method: "POST",
      path: "/curriculum-map/nodes/node-1/verify",
    },
    {
      name: "curriculum map retire",
      op: { kind: "retireCurriculumMapNode", id: "node-1" },
      method: "POST",
      path: "/curriculum-map/nodes/node-1/retire",
    },
  ];

  it.each(cases)("builds only the fixed Core URL for $name", async ({ op, method, path }) => {
    mockSignedIn("secret-token-value");
    fetchSpy.mockResolvedValue(jsonResponse(200, { ok: true }));

    await callCore(op);

    expect(fetchSpy).toHaveBeenCalledTimes(1);
    const [calledUrl, calledInit] = fetchSpy.mock.calls[0];
    expect(calledUrl).toBe(`http://core.internal:8080${path}`);
    expect(calledInit.method).toBe(method);
    expect(calledInit.headers.Authorization).toBe("Bearer secret-token-value");
  });

  it("passes Core's status and body through unchanged, including 403", async () => {
    mockSignedIn();
    fetchSpy.mockResolvedValue(jsonResponse(403, { error: "forbidden", message: "denied" }));

    const result = await callCore({ kind: "listSyllabuses" });

    expect(result.status).toBe(403);
    expect(result.body).toEqual({ error: "forbidden", message: "denied" });
  });

  it("maps a network failure to a generic 502 without leaking the underlying error", async () => {
    mockSignedIn();
    fetchSpy.mockRejectedValue(new Error("ECONNREFUSED 10.0.0.1:8080"));

    const err = await callCore({ kind: "listSyllabuses" }).catch((e) => e);

    expect(err).toBeInstanceOf(ProxyError);
    expect((err as ProxyError).status).toBe(502);
    expect((err as ProxyError).message).not.toContain("ECONNREFUSED");
    expect((err as ProxyError).message).not.toContain("10.0.0.1");
  });

  it("maps a non-JSON upstream response to a generic 502", async () => {
    mockSignedIn();
    fetchSpy.mockResolvedValue(new Response("<html>not json</html>", { status: 200 }));

    const err = await callCore({ kind: "listSyllabuses" }).catch((e) => e);

    expect(err).toBeInstanceOf(ProxyError);
    expect((err as ProxyError).status).toBe(502);
  });

  it("maps a Core 500 to a safe generic 502 and never exposes the raw body", async () => {
    mockSignedIn();
    fetchSpy.mockResolvedValue(
      jsonResponse(500, {
        error: "internal_error",
        message: "pq: connection to server at \"10.0.0.5\" failed: dial tcp 10.0.0.5:5432",
      }),
    );

    const err = await callCore({ kind: "listSyllabuses" }).catch((e) => e);

    expect(err).toBeInstanceOf(ProxyError);
    expect((err as ProxyError).status).toBe(502);
    expect((err as ProxyError).code).toBe("upstream_error");
    expect((err as ProxyError).message).not.toContain("10.0.0.5");
    expect((err as ProxyError).message).not.toContain("pq:");
    expect((err as ProxyError).message).not.toContain("dial tcp");
  });

  it.each([502, 503])("maps a Core %d to the same safe generic 502", async (upstreamStatus) => {
    mockSignedIn();
    fetchSpy.mockResolvedValue(jsonResponse(upstreamStatus, { error: "bad_gateway", message: "db down" }));

    const err = await callCore({ kind: "listSyllabuses" }).catch((e) => e);

    expect(err).toBeInstanceOf(ProxyError);
    expect((err as ProxyError).status).toBe(502);
    expect((err as ProxyError).code).toBe("upstream_error");
    expect((err as ProxyError).message).not.toContain("db down");
  });

  it.each([400, 401, 403, 409])("passes Core's safe %d body and status through unchanged", async (status) => {
    mockSignedIn();
    fetchSpy.mockResolvedValue(jsonResponse(status, { error: "domain_error", message: "safe message" }));

    const result = await callCore({ kind: "listSyllabuses" });

    expect(result.status).toBe(status);
    expect(result.body).toEqual({ error: "domain_error", message: "safe message" });
  });

  it("fails closed on a Core redirect without following it or requesting the redirect target", async () => {
    mockSignedIn("secret-token-value");
    fetchSpy.mockImplementation(async (_url, init: RequestInit) => {
      expect(init.redirect).toBe("error");
      throw new TypeError("Failed to fetch");
    });

    const err = await callCore({ kind: "listSyllabuses" }).catch((e) => e);

    expect(err).toBeInstanceOf(ProxyError);
    expect((err as ProxyError).status).toBe(502);
    expect(fetchSpy).toHaveBeenCalledTimes(1);
  });

  it("never logs the bearer token or forwards it in any thrown message", async () => {
    mockSignedIn("super-secret-token");
    fetchSpy.mockRejectedValue(new Error("boom"));

    await callCore({ kind: "listSyllabuses" }).catch(() => {});

    expect(logSpy).not.toHaveBeenCalled();
    expect(errorSpy).not.toHaveBeenCalled();
    const allLoggedText = [...logSpy.mock.calls, ...errorSpy.mock.calls].flat().join(" ");
    expect(allLoggedText).not.toContain("super-secret-token");
  });
});

describe("readSafeJsonBody", () => {
  it("rejects a non-JSON content-type", async () => {
    const request = new Request("http://localhost/x", {
      method: "POST",
      headers: { "content-type": "text/plain" },
      body: "{}",
    });

    const err = await readSafeJsonBody(request).catch((e) => e);

    expect(err).toBeInstanceOf(ProxyError);
    expect((err as ProxyError).status).toBe(400);
  });

  it("rejects a body over the size limit", async () => {
    const big = JSON.stringify({ title: "a".repeat(200_000) });
    const request = new Request("http://localhost/x", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: big,
    });

    const err = await readSafeJsonBody(request).catch((e) => e);

    expect(err).toBeInstanceOf(ProxyError);
    expect((err as ProxyError).status).toBe(413);
  });

  it("rejects syntactically invalid JSON", async () => {
    const request = new Request("http://localhost/x", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: "{not json",
    });

    const err = await readSafeJsonBody(request).catch((e) => e);

    expect(err).toBeInstanceOf(ProxyError);
    expect((err as ProxyError).status).toBe(400);
  });

  it("returns the raw text unchanged for valid JSON", async () => {
    const raw = '{"title":"Photosynthesis","sourceUrl":"https://example.org/x"}';
    const request = new Request("http://localhost/x", {
      method: "POST",
      headers: { "content-type": "application/json; charset=utf-8" },
      body: raw,
    });

    const result = await readSafeJsonBody(request);

    expect(result).toBe(raw);
  });
});
