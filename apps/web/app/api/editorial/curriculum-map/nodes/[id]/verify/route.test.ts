// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from "vitest";
import { callCore } from "@/lib/editorial/core-proxy";
import { POST } from "./route";

vi.mock("@/lib/editorial/core-proxy", async () => {
  const actual = await vi.importActual<typeof import("@/lib/editorial/core-proxy")>(
    "@/lib/editorial/core-proxy",
  );
  return { ...actual, callCore: vi.fn() };
});

const mockedCallCore = vi.mocked(callCore);

beforeEach(() => {
  vi.clearAllMocks();
});

describe("route exports", () => {
  it("exposes exactly POST — no other method is handled", async () => {
    const mod = await import("./route");
    expect(Object.keys(mod).sort()).toEqual(["POST"]);
  });
});

describe("POST /api/editorial/curriculum-map/nodes/[id]/verify", () => {
  it("ignores any client-supplied body and sends a fixed empty object to Core", async () => {
    mockedCallCore.mockResolvedValue({ status: 200, body: { id: "node-1", status: "verified" } });

    const response = await POST(
      new Request("http://localhost/x", {
        method: "POST",
        body: JSON.stringify({ status: "verified" }),
      }),
      { params: Promise.resolve({ id: "node-1" }) },
    );

    expect(mockedCallCore).toHaveBeenCalledWith({ kind: "verifyCurriculumMapNode", id: "node-1" }, "{}");
    expect(response.status).toBe(200);
  });
});
