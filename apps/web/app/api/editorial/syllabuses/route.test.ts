// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from "vitest";
import { callCore } from "@/lib/editorial/core-proxy";
import { GET } from "./route";

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
  it("exposes exactly GET — no other method is handled", async () => {
    const mod = await import("./route");
    expect(Object.keys(mod).sort()).toEqual(["GET"]);
  });
});

describe("GET /api/editorial/syllabuses", () => {
  it("returns Core's syllabus list unchanged", async () => {
    mockedCallCore.mockResolvedValue({ status: 200, body: { items: [{ id: "s1" }] } });

    const response = await GET();

    expect(mockedCallCore).toHaveBeenCalledWith({ kind: "listSyllabuses" });
    expect(await response.json()).toEqual({ items: [{ id: "s1" }] });
  });
});
