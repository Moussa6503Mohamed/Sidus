// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from "vitest";
import { auth } from "@clerk/nextjs/server";
import { callCoreLearner } from "@/lib/learner/core-proxy";
import { POST as createAttempt } from "./questions/[id]/attempts/route";
import { POST as submitAttempt } from "./attempts/[id]/submit/route";

vi.mock("@/lib/learner/core-proxy", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/learner/core-proxy")>();
  return { ...actual, callCoreLearner: vi.fn() };
});
vi.mock("@clerk/nextjs/server", () => ({ auth: vi.fn() }));
const call = vi.mocked(callCoreLearner);
const mockedAuth = vi.mocked(auth);
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

  it.each([
    ["create", createAttempt, params("q-1"), "   "],
    ["submit", submitAttempt, params("a-1"), `{"selectedOptionId":"opt-a"}`],
  ])("rejects oversized declared %s body before auth or Core fetch", async (_name, handler, routeParams, body) => {
    const fetchSpy = vi.spyOn(global, "fetch");
    const response = await handler(new Request("http://local", {
      method: "POST",
      headers: { "Content-Length": "4097" },
      body,
    }), routeParams);

    expect(response.status).toBe(413);
    expect(call).not.toHaveBeenCalled();
    expect(mockedAuth).not.toHaveBeenCalled();
    expect(fetchSpy).not.toHaveBeenCalled();
    fetchSpy.mockRestore();
  });

  it.each([
    ["create", createAttempt, params("q-1"), " ".repeat(4097)],
    ["submit", submitAttempt, params("a-1"), `{"selectedOptionId":"${"a".repeat(4097)}"}`],
  ])("caps chunked or untrusted %s body before auth or Core fetch", async (_name, handler, routeParams, body) => {
    const fetchSpy = vi.spyOn(global, "fetch");
    const stream = new ReadableStream<Uint8Array>({
      start(controller) {
        const bytes = new TextEncoder().encode(body);
        controller.enqueue(bytes.slice(0, 2048));
        controller.enqueue(bytes.slice(2048));
        controller.close();
      },
    });
    const response = await handler(new Request("http://local", {
      method: "POST",
      body: stream,
      duplex: "half",
    } as RequestInit & { duplex: "half" }), routeParams);

    expect(response.status).toBe(413);
    expect(call).not.toHaveBeenCalled();
    expect(mockedAuth).not.toHaveBeenCalled();
    expect(fetchSpy).not.toHaveBeenCalled();
    fetchSpy.mockRestore();
  });
});
