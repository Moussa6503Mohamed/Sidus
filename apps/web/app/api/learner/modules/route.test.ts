// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from "vitest";
import { callCoreLearner, LearnerProxyError } from "@/lib/learner/core-proxy";
import { GET } from "./route";

vi.mock("@/lib/learner/core-proxy", async () => {
  const actual = await vi.importActual<typeof import("@/lib/learner/core-proxy")>("@/lib/learner/core-proxy");
  return { ...actual, callCoreLearner: vi.fn() };
});

const mockedCallCoreLearner = vi.mocked(callCoreLearner);
beforeEach(() => vi.clearAllMocks());

describe("GET /api/learner/modules", () => {
  it("exports only GET", async () => {
    expect(Object.keys(await import("./route")).sort()).toEqual(["GET"]);
  });
  it("forwards only syllabusId", async () => {
    mockedCallCoreLearner.mockResolvedValue({ status: 200, body: { items: [] } });
    const response = await GET(new Request("http://localhost/api/learner/modules?syllabusId=syl-1"));
    expect(mockedCallCoreLearner).toHaveBeenCalledWith({ kind: "listLearnerModules", syllabusId: "syl-1" });
    expect(response.status).toBe(200);
  });
  it("fails before Core when syllabusId is absent", async () => {
    const response = await GET(new Request("http://localhost/api/learner/modules"));
    expect(response.status).toBe(400);
    expect(mockedCallCoreLearner).not.toHaveBeenCalled();
  });
  it("preserves safe proxy errors", async () => {
    mockedCallCoreLearner.mockRejectedValue(new LearnerProxyError(401, "unauthorized", "sign-in is required"));
    const response = await GET(new Request("http://localhost/api/learner/modules?syllabusId=syl-1"));
    expect(response.status).toBe(401);
  });
});
