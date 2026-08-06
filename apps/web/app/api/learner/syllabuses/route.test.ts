// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from "vitest";
import { callCoreLearner, LearnerProxyError } from "@/lib/learner/core-proxy";
import { GET } from "./route";

vi.mock("@/lib/learner/core-proxy", async () => {
  const actual = await vi.importActual<typeof import("@/lib/learner/core-proxy")>(
    "@/lib/learner/core-proxy",
  );
  return {
    ...actual,
    callCoreLearner: vi.fn(),
  };
});

const mockedCallCoreLearner = vi.mocked(callCoreLearner);

beforeEach(() => {
  vi.clearAllMocks();
});

describe("route exports", () => {
  it("exposes exactly GET — no write method is handled", async () => {
    const mod = await import("./route");
    expect(Object.keys(mod).sort()).toEqual(["GET"]);
  });
});

describe("GET /api/learner/syllabuses", () => {
  it("calls the fixed listLearnerSyllabuses operation and forwards Core's response", async () => {
    mockedCallCoreLearner.mockResolvedValue({
      status: 200,
      body: { items: [{ id: "syl-1", board: "Cambridge", syllabusCode: "0610", qualification: "IGCSE", track: "Extended", displayName: "Biology 0610 Extended" }] },
    });

    const response = await GET();

    expect(mockedCallCoreLearner).toHaveBeenCalledWith({ kind: "listLearnerSyllabuses" });
    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({
      items: [{ id: "syl-1", board: "Cambridge", syllabusCode: "0610", qualification: "IGCSE", track: "Extended", displayName: "Biology 0610 Extended" }],
    });
  });

  it("maps a LearnerProxyError to its status and code", async () => {
    mockedCallCoreLearner.mockRejectedValue(new LearnerProxyError(503, "service_unavailable", "not configured"));

    const response = await GET();

    expect(response.status).toBe(503);
    expect(await response.json()).toEqual({ error: "service_unavailable", message: "not configured" });
  });
});
