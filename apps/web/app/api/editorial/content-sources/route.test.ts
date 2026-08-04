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
    const exported = Object.keys(mod).sort();
    expect(exported).toEqual(["GET", "POST"]);
  });
});

describe("GET /api/editorial/content-sources", () => {
  it("forwards the status query param and Core's response", async () => {
    mockedCallCore.mockResolvedValue({ status: 200, body: { items: [] } });

    const response = await GET(new Request("http://localhost/api/editorial/content-sources?status=pending"));

    expect(mockedCallCore).toHaveBeenCalledWith({ kind: "listContentSources", status: "pending" });
    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({ items: [] });
  });

  it("maps a ProxyError to its status and code", async () => {
    mockedCallCore.mockRejectedValue(new ProxyError(503, "service_unavailable", "not configured"));

    const response = await GET(new Request("http://localhost/api/editorial/content-sources"));

    expect(response.status).toBe(503);
    expect(await response.json()).toEqual({ error: "service_unavailable", message: "not configured" });
  });
});

describe("POST /api/editorial/content-sources", () => {
  it("reads the body then creates via Core", async () => {
    mockedReadBody.mockResolvedValue('{"title":"x","sourceUrl":"https://example.org"}');
    mockedCallCore.mockResolvedValue({ status: 201, body: { id: "1" } });

    const response = await POST(
      new Request("http://localhost/api/editorial/content-sources", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: '{"title":"x","sourceUrl":"https://example.org"}',
      }),
    );

    expect(mockedCallCore).toHaveBeenCalledWith(
      { kind: "createContentSource" },
      '{"title":"x","sourceUrl":"https://example.org"}',
    );
    expect(response.status).toBe(201);
  });

  it("never calls Core when the body fails validation", async () => {
    mockedReadBody.mockRejectedValue(new ProxyError(400, "invalid_json", "bad body"));

    const response = await POST(
      new Request("http://localhost/api/editorial/content-sources", { method: "POST", body: "{" }),
    );

    expect(response.status).toBe(400);
    expect(mockedCallCore).not.toHaveBeenCalled();
  });
});
