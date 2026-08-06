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

describe("GET /api/learner/questions", () => {
  it("forwards syllabusId and curriculumMapNodeId and Core's response", async () => {
    mockedCallCoreLearner.mockResolvedValue({ status: 200, body: { items: [] } });

    const response = await GET(
      new Request("http://localhost/api/learner/questions?syllabusId=syl-1&curriculumMapNodeId=node-1"),
    );

    expect(mockedCallCoreLearner).toHaveBeenCalledWith({
      kind: "listLearnerQuestions",
      syllabusId: "syl-1",
      curriculumMapNodeId: "node-1",
    });
    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({ items: [] });
  });

  it("returns 400 and never calls Core when syllabusId is missing", async () => {
    const response = await GET(new Request("http://localhost/api/learner/questions"));

    expect(response.status).toBe(400);
    expect(mockedCallCoreLearner).not.toHaveBeenCalled();
  });

  it("maps a LearnerProxyError to its status and code", async () => {
    mockedCallCoreLearner.mockRejectedValue(new LearnerProxyError(503, "service_unavailable", "not configured"));

    const response = await GET(new Request("http://localhost/api/learner/questions?syllabusId=syl-1"));

    expect(response.status).toBe(503);
    expect(await response.json()).toEqual({ error: "service_unavailable", message: "not configured" });
  });
});
