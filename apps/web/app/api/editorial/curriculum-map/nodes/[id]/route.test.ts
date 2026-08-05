// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from "vitest";
import { callCore, ProxyError, readSafeJsonBody } from "@/lib/editorial/core-proxy";
import { GET, PATCH } from "./route";

vi.mock("@/lib/editorial/core-proxy", async () => {
  const actual = await vi.importActual<typeof import("@/lib/editorial/core-proxy")>(
    "@/lib/editorial/core-proxy",
  );
  return { ...actual, callCore: vi.fn(), readSafeJsonBody: vi.fn() };
});

const mockedCallCore = vi.mocked(callCore);
const mockedReadBody = vi.mocked(readSafeJsonBody);

beforeEach(() => {
  vi.clearAllMocks();
});

describe("route exports", () => {
  it("exposes exactly GET and PATCH — no other method is handled", async () => {
    const mod = await import("./route");
    expect(Object.keys(mod).sort()).toEqual(["GET", "PATCH"]);
  });
});

describe("GET /api/editorial/curriculum-map/nodes/[id]", () => {
  it("passes the resolved id through to Core", async () => {
    mockedCallCore.mockResolvedValue({ status: 200, body: { id: "node-1" } });

    const response = await GET(new Request("http://localhost/x"), {
      params: Promise.resolve({ id: "node-1" }),
    });

    expect(mockedCallCore).toHaveBeenCalledWith({ kind: "getCurriculumMapNode", id: "node-1" });
    expect(response.status).toBe(200);
  });

  it("maps a not-found ProxyError through unchanged", async () => {
    mockedCallCore.mockRejectedValue(new ProxyError(404, "not_found", "curriculum map node not found"));

    const response = await GET(new Request("http://localhost/x"), {
      params: Promise.resolve({ id: "missing" }),
    });

    expect(response.status).toBe(404);
  });
});

describe("PATCH /api/editorial/curriculum-map/nodes/[id]", () => {
  it("forwards the validated raw body", async () => {
    mockedReadBody.mockResolvedValue('{"label":"new"}');
    mockedCallCore.mockResolvedValue({ status: 200, body: { id: "node-1", label: "new" } });

    const response = await PATCH(
      new Request("http://localhost/x", { method: "PATCH", body: '{"label":"new"}' }),
      { params: Promise.resolve({ id: "node-1" }) },
    );

    expect(mockedCallCore).toHaveBeenCalledWith({ kind: "updateCurriculumMapNode", id: "node-1" }, '{"label":"new"}');
    expect(response.status).toBe(200);
  });

  it("never calls Core when the id is rejected by the proxy", async () => {
    mockedReadBody.mockResolvedValue("{}");
    mockedCallCore.mockRejectedValue(new ProxyError(400, "invalid_id", "identifier is not a valid resource id"));

    const response = await PATCH(new Request("http://localhost/x", { method: "PATCH", body: "{}" }), {
      params: Promise.resolve({ id: "../evil" }),
    });

    expect(response.status).toBe(400);
  });
});
