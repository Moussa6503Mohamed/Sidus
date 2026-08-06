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
  it("exposes exactly GET", async () => {
    const mod = await import("./route");
    expect(Object.keys(mod).sort()).toEqual(["GET"]);
  });
});

describe("GET /api/learner/questions/[id]", () => {
  it("forwards the id and Core's response", async () => {
    mockedCallCoreLearner.mockResolvedValue({ status: 200, body: { id: "q-1" } });

    const response = await GET(new Request("http://localhost/api/learner/questions/q-1"), {
      params: Promise.resolve({ id: "q-1" }),
    });

    expect(mockedCallCoreLearner).toHaveBeenCalledWith({ kind: "getLearnerQuestion", id: "q-1" });
    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({ id: "q-1" });
  });

  it("maps a not-found LearnerProxyError-equivalent Core body through unchanged", async () => {
    mockedCallCoreLearner.mockResolvedValue({
      status: 404,
      body: { error: "not_found", message: "question not found" },
    });

    const response = await GET(new Request("http://localhost/api/learner/questions/missing"), {
      params: Promise.resolve({ id: "missing" }),
    });

    expect(response.status).toBe(404);
  });

  it("maps a LearnerProxyError to its status and code", async () => {
    mockedCallCoreLearner.mockRejectedValue(new LearnerProxyError(401, "unauthorized", "sign-in is required"));

    const response = await GET(new Request("http://localhost/api/learner/questions/q-1"), {
      params: Promise.resolve({ id: "q-1" }),
    });

    expect(response.status).toBe(401);
    expect(await response.json()).toEqual({ error: "unauthorized", message: "sign-in is required" });
  });
});
