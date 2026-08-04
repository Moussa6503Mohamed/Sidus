// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from "vitest";
import { callCore, readSafeJsonBody } from "@/lib/editorial/core-proxy";
import { POST } from "./route";

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
  it("exposes exactly POST — no other method is handled", async () => {
    const mod = await import("./route");
    expect(Object.keys(mod).sort()).toEqual(["POST"]);
  });
});

describe("POST /api/editorial/content-sources/[id]/reject", () => {
  it("forwards the validated reason body", async () => {
    mockedReadBody.mockResolvedValue('{"reason":"not licensed"}');
    mockedCallCore.mockResolvedValue({ status: 200, body: { status: "rejected" } });

    const response = await POST(
      new Request("http://localhost/x", { method: "POST", body: '{"reason":"not licensed"}' }),
      { params: Promise.resolve({ id: "abc-1" }) },
    );

    expect(mockedCallCore).toHaveBeenCalledWith(
      { kind: "rejectContentSource", id: "abc-1" },
      '{"reason":"not licensed"}',
    );
    expect(response.status).toBe(200);
  });
});
