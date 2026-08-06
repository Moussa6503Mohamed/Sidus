// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from "vitest";
import { callCoreLearner } from "@/lib/learner/core-proxy";
import { POST as createAttempt } from "./questions/[id]/attempts/route";
import { POST as submitAttempt } from "./attempts/[id]/submit/route";

vi.mock("@/lib/learner/core-proxy", () => ({ callCoreLearner: vi.fn() }));
const call = vi.mocked(callCoreLearner);
const params = (id: string) => ({ params: Promise.resolve({ id }) });

beforeEach(() => { vi.clearAllMocks(); });

describe("learner attempt BFF routes", () => {
  it("exposes fixed create and submit operations", async () => {
    call.mockResolvedValueOnce({ status: 201, body: { attemptId: "a-1" } });
    const created = await createAttempt(new Request("http://local", { method: "POST" }), params("q-1"));
    expect(created.status).toBe(201);
    expect(call).toHaveBeenCalledWith({ kind: "createLearnerAttempt", questionId: "q-1" });

    call.mockResolvedValueOnce({ status: 200, body: { attemptId: "a-1" } });
    const submitted = await submitAttempt(new Request("http://local", { method: "POST", body: `{"selectedOptionId":"opt-a"}` }), params("a-1"));
    expect(submitted.status).toBe(200);
    expect(call).toHaveBeenLastCalledWith({ kind: "submitLearnerAttempt", attemptId: "a-1", selectedOptionId: "opt-a" });
  });

  it.each([`{}`, `null`, `{"SelectedOptionId":"opt-a"}`, `{"selectedOptionId":""}`, `{"selectedOptionId":"opt-a","extra":1}`, `{"selectedOptionId":"opt-a","selectedOptionId":"opt-b"}`])(
    "rejects malformed submit body before Core proxy: %s", async (body) => {
      const response = await submitAttempt(new Request("http://local", { method: "POST", body }), params("a-1"));
      expect(response.status).toBe(400);
      expect(call).not.toHaveBeenCalled();
    },
  );

  it("rejects create body before Core proxy", async () => {
    const response = await createAttempt(new Request("http://local", { method: "POST", body: `{}` }), params("q-1"));
    expect(response.status).toBe(400);
    expect(call).not.toHaveBeenCalled();
  });
});
