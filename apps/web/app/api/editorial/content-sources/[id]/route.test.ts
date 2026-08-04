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

describe("GET /api/editorial/content-sources/[id]", () => {
  it("passes the resolved id through to Core", async () => {
    mockedCallCore.mockResolvedValue({ status: 200, body: { id: "abc-1" } });

    const response = await GET(new Request("http://localhost/x"), {
      params: Promise.resolve({ id: "abc-1" }),
    });

    expect(mockedCallCore).toHaveBeenCalledWith({ kind: "getContentSource", id: "abc-1" });
    expect(response.status).toBe(200);
  });
});

describe("PATCH /api/editorial/content-sources/[id]", () => {
  it("forwards the validated raw body", async () => {
    mockedReadBody.mockResolvedValue('{"title":"new"}');
    mockedCallCore.mockResolvedValue({ status: 200, body: { id: "abc-1", title: "new" } });

    const response = await PATCH(
      new Request("http://localhost/x", { method: "PATCH", body: '{"title":"new"}' }),
      { params: Promise.resolve({ id: "abc-1" }) },
    );

    expect(mockedCallCore).toHaveBeenCalledWith({ kind: "updateContentSource", id: "abc-1" }, '{"title":"new"}');
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
