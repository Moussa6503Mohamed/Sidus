// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { callTeacherCore } from "@/lib/teacher/core-proxy";
import { POST as createClass } from "./classes/route";
import { POST as createInvite } from "./classes/[id]/invites/route";
import { POST as createAssignment } from "./classes/[id]/assignments/route";

vi.mock("@/lib/teacher/core-proxy", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/teacher/core-proxy")>();
  return { ...actual, callTeacherCore: vi.fn() };
});

const mockedCore = vi.mocked(callTeacherCore);
const params = { params: Promise.resolve({ id: "class_1" }) };

function chunked(bytes: number): Request {
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      controller.enqueue(new TextEncoder().encode("{" + " ".repeat(4095)));
      controller.enqueue(new TextEncoder().encode(" ".repeat(bytes - 4096) + "}"));
      controller.close();
    },
  });
  return new Request("http://sidus.test", { method: "POST", body: stream, duplex: "half" } as RequestInit);
}

afterEach(() => vi.clearAllMocks());

describe("teacher mutation routes", () => {
  it("rejects an oversized chunked class body before the Clerk/Core boundary", async () => {
    const response = await createClass(chunked(8193));
    expect(response.status).toBe(413);
    expect(await response.json()).toEqual({ error: "payload_too_large" });
    expect(mockedCore).not.toHaveBeenCalled();
  });

  it("rejects a declared oversized body before the Clerk/Core boundary", async () => {
    const response = await createClass(new Request("http://sidus.test", { method: "POST", headers: { "content-length": "8193" }, body: "{}" }));
    expect(response.status).toBe(413);
    expect(mockedCore).not.toHaveBeenCalled();
  });

  it("uses exact schemas before the Clerk/Core boundary for every teacher mutation", async () => {
    const classResponse = await createClass(new Request("http://sidus.test", { method: "POST", body: '{"name":"x","extra":true}' }));
    const inviteResponse = await createInvite(new Request("http://sidus.test", { method: "POST", body: '{"ttlSeconds":60}' }), params);
    const assignmentResponse = await createAssignment(new Request("http://sidus.test", { method: "POST", body: '{"title":"x","syllabusId":"s","moduleId":"m","questionIds":["q"],"markingMode":"bad"}' }), params);
    expect([classResponse.status, inviteResponse.status, assignmentResponse.status]).toEqual([400, 400, 400]);
    expect(mockedCore).not.toHaveBeenCalled();
  });

  it("rejects duplicate escaped schema keys before the Clerk/Core boundary", async () => {
    const response = await createClass(new Request("http://sidus.test", { method: "POST", body: '{"name":"first","na\\u006de":"second"}' }));
    expect(response.status).toBe(400);
    expect(await response.json()).toEqual({ error: "invalid_json" });
    expect(mockedCore).not.toHaveBeenCalled();
  });

  it("passes a validated canonical class body to the fixed Core operation", async () => {
    mockedCore.mockResolvedValue({ status: 201, body: { id: "class_1" } });
    const response = await createClass(new Request("http://sidus.test", { method: "POST", body: '{"name":"  Biology  "}' }));
    expect(response.status).toBe(201);
    expect(mockedCore).toHaveBeenCalledWith({ kind: "createClass", body: '{"name":"Biology"}' });
  });
});
