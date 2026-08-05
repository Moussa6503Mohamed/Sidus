// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from "vitest";
import { callCore, ProxyError, readSafeJsonBody } from "@/lib/editorial/core-proxy";
import { GET, POST } from "./route";

vi.mock("@/lib/editorial/core-proxy", async () => {
  const actual = await vi.importActual<typeof import("@/lib/editorial/core-proxy")>(
    "@/lib/editorial/core-proxy",
  );
  return {
    ...actual,
    callCore: vi.fn(),
    readSafeJsonBody: vi.fn(),
  };
});

const mockedCallCore = vi.mocked(callCore);
const mockedReadBody = vi.mocked(readSafeJsonBody);

beforeEach(() => {
  vi.clearAllMocks();
});

describe("route exports", () => {
  it("exposes exactly GET and POST — no other method is handled", async () => {
    const mod = await import("./route");
    expect(Object.keys(mod).sort()).toEqual(["GET", "POST"]);
  });
});

describe("GET /api/editorial/curriculum-map/nodes", () => {
  it("forwards syllabusId and status query params to Core", async () => {
    mockedCallCore.mockResolvedValue({ status: 200, body: { items: [] } });

    const response = await GET(
      new Request("http://localhost/api/editorial/curriculum-map/nodes?syllabusId=syl-1&status=draft"),
    );

    expect(mockedCallCore).toHaveBeenCalledWith({
      kind: "listCurriculumMapNodes",
      syllabusId: "syl-1",
      status: "draft",
    });
    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({ items: [] });
  });

  it("returns stable invalid_id when syllabusId is omitted", async () => {
    mockedCallCore.mockRejectedValue(
      new ProxyError(400, "invalid_id", "identifier is not a valid resource id"),
    );

    const response = await GET(new Request("http://localhost/api/editorial/curriculum-map/nodes"));

    expect(mockedCallCore).toHaveBeenCalledWith({
      kind: "listCurriculumMapNodes",
      syllabusId: "",
      status: undefined,
    });
    expect(response.status).toBe(400);
    expect(await response.json()).toEqual({
      error: "invalid_id",
      message: "identifier is not a valid resource id",
    });
  });

  it("forwards explicit status=all through the fixed curriculum-map operation", async () => {
    mockedCallCore.mockResolvedValue({ status: 200, body: { items: [] } });

    const response = await GET(
      new Request("http://localhost/api/editorial/curriculum-map/nodes?syllabusId=syl-1&status=all"),
    );

    expect(mockedCallCore).toHaveBeenCalledWith({
      kind: "listCurriculumMapNodes",
      syllabusId: "syl-1",
      status: "all",
    });
    expect(response.status).toBe(200);
  });

  it("maps a ProxyError to its status and code", async () => {
    mockedCallCore.mockRejectedValue(new ProxyError(503, "service_unavailable", "not configured"));

    const response = await GET(
      new Request("http://localhost/api/editorial/curriculum-map/nodes?syllabusId=syl-1"),
    );

    expect(response.status).toBe(503);
    expect(await response.json()).toEqual({ error: "service_unavailable", message: "not configured" });
  });
});

describe("POST /api/editorial/curriculum-map/nodes", () => {
  it("reads the body then creates via Core", async () => {
    const body = '{"syllabusId":"syl-1","nodeKind":"topic","nodeCode":"T1","label":"Topic","contentSourceId":"src-1"}';
    mockedReadBody.mockResolvedValue(body);
    mockedCallCore.mockResolvedValue({ status: 201, body: { id: "node-1" } });

    const response = await POST(
      new Request("http://localhost/api/editorial/curriculum-map/nodes", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body,
      }),
    );

    expect(mockedCallCore).toHaveBeenCalledWith({ kind: "createCurriculumMapNode" }, body);
    expect(response.status).toBe(201);
  });

  it("never calls Core when the body fails validation", async () => {
    mockedReadBody.mockRejectedValue(new ProxyError(400, "invalid_json", "bad body"));

    const response = await POST(
      new Request("http://localhost/api/editorial/curriculum-map/nodes", { method: "POST", body: "{" }),
    );

    expect(response.status).toBe(400);
    expect(mockedCallCore).not.toHaveBeenCalled();
  });
});
