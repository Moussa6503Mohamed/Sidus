// @vitest-environment node
import { beforeEach, expect, it, vi } from "vitest";
import { callCoreLearner, LearnerProxyError } from "@/lib/learner/core-proxy";
import { GET } from "./route";

vi.mock("@/lib/learner/core-proxy", async () => ({ ...(await vi.importActual<typeof import("@/lib/learner/core-proxy")>("@/lib/learner/core-proxy")), callCoreLearner: vi.fn() }));
const mocked = vi.mocked(callCoreLearner);
beforeEach(() => vi.clearAllMocks());
it("uses only the fixed analytics operation", async () => {
  mocked.mockResolvedValue({ status: 200, body: { scoredItems: 0, awardedMarks: 0, possibleMarks: 0, pendingMarking: 0, withheldMarking: 0, syllabuses: [], modules: [], recentActivity: [] } });
  const response = await GET();
  expect(mocked).toHaveBeenCalledWith({ kind: "getLearnerAnalytics" });
  expect(response.status).toBe(200);
});
it("maps a safe proxy error", async () => {
  mocked.mockRejectedValue(new LearnerProxyError(503, "service_unavailable", "not configured"));
  const response = await GET();
  expect(response.status).toBe(503);
});
