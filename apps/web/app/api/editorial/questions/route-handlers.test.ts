// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from "vitest";
import { callCore, ProxyError, readCanonicalRubricBody, readSafeJsonBody } from "@/lib/editorial/core-proxy";
import { GET as list, POST as create } from "./route";
import { GET as get, PATCH as update } from "./[id]/route";
import { POST as verify } from "./[id]/verify/route";
import { POST as setCanonical } from "./[id]/canonical-rubric/route";
import { POST as retire } from "./[id]/retire/route";
import { GET as listRubrics, POST as createRubric } from "./[id]/rubric-versions/route";
import { POST as verifyRubric } from "./[id]/rubric-versions/[version]/verify/route";

vi.mock("@/lib/editorial/core-proxy", async () => {
  const actual = await vi.importActual<typeof import("@/lib/editorial/core-proxy")>("@/lib/editorial/core-proxy");
  return { ...actual, callCore: vi.fn(), readSafeJsonBody: vi.fn(), readCanonicalRubricBody: vi.fn() };
});

const mockedCallCore = vi.mocked(callCore);
const mockedReadBody = vi.mocked(readSafeJsonBody);
const mockedReadCanonicalBody = vi.mocked(readCanonicalRubricBody);
const idParams = { params: Promise.resolve({ id: "question-1" }) };

beforeEach(() => {
  vi.clearAllMocks();
  mockedCallCore.mockResolvedValue({ status: 200, body: {} });
  mockedReadBody.mockResolvedValue("{}");
  mockedReadCanonicalBody.mockResolvedValue(`{"rubricVersion":2}`);
});

describe("question route method allowlist", () => {
  it("exports only intended methods", async () => {
    expect(Object.keys(await import("./route")).sort()).toEqual(["GET", "POST"]);
    expect(Object.keys(await import("./[id]/route")).sort()).toEqual(["GET", "PATCH"]);
    expect(Object.keys(await import("./[id]/verify/route"))).toEqual(["POST"]);
    expect(Object.keys(await import("./[id]/canonical-rubric/route"))).toEqual(["POST"]);
    expect(Object.keys(await import("./[id]/retire/route"))).toEqual(["POST"]);
    expect(Object.keys(await import("./[id]/rubric-versions/route")).sort()).toEqual(["GET", "POST"]);
    expect(Object.keys(await import("./[id]/rubric-versions/[version]/verify/route"))).toEqual(["POST"]);
  });
});

describe("question route delegation", () => {
  it("delegates list query and create body", async () => {
    await list(new Request("http://localhost/api/editorial/questions?syllabusId=syl-1&curriculumMapNodeId=node-1&status=all"));
    expect(mockedCallCore).toHaveBeenCalledWith({
      kind: "listQuestions",
      syllabusId: "syl-1",
      curriculumMapNodeId: "node-1",
      status: "all",
    });

    await create(new Request("http://localhost/x", { method: "POST", body: "{}" }));
    expect(mockedReadBody).toHaveBeenCalledTimes(1);
    expect(mockedCallCore).toHaveBeenLastCalledWith({ kind: "createQuestion" }, "{}");
  });

  it("delegates get and patch using resolved id", async () => {
    await get(new Request("http://localhost/x"), idParams);
    expect(mockedCallCore).toHaveBeenCalledWith({ kind: "getQuestion", id: "question-1" });

    await update(new Request("http://localhost/x", { method: "PATCH", body: "{}" }), idParams);
    expect(mockedCallCore).toHaveBeenLastCalledWith({ kind: "updateQuestion", id: "question-1" }, "{}");
  });

  it("delegates canonical selection bodies and fixed retirement body", async () => {
    const request = new Request("http://localhost/x", { method: "POST", body: "ignored" });
    await verify(request, idParams);
    expect(mockedCallCore).toHaveBeenCalledWith({ kind: "verifyQuestion", id: "question-1" }, `{"rubricVersion":2}`);
    await setCanonical(new Request("http://localhost/x", { method: "POST", body: "ignored" }), idParams);
    expect(mockedCallCore).toHaveBeenLastCalledWith({ kind: "setQuestionCanonicalRubric", id: "question-1" }, `{"rubricVersion":2}`);
    await retire(request, idParams);
    expect(mockedCallCore).toHaveBeenLastCalledWith({ kind: "retireQuestion", id: "question-1" }, "{}");
    expect(mockedReadCanonicalBody).toHaveBeenCalledTimes(2);
  });

  it("delegates rubric list, create, and verify", async () => {
    await listRubrics(new Request("http://localhost/x"), idParams);
    expect(mockedCallCore).toHaveBeenCalledWith({ kind: "listQuestionRubricVersions", id: "question-1" });
    await createRubric(new Request("http://localhost/x", { method: "POST", body: "{}" }), idParams);
    expect(mockedCallCore).toHaveBeenLastCalledWith({ kind: "createQuestionRubricVersion", id: "question-1" }, "{}");
    await verifyRubric(new Request("http://localhost/x", { method: "POST" }), {
      params: Promise.resolve({ id: "question-1", version: "2" }),
    });
    expect(mockedCallCore).toHaveBeenLastCalledWith({
      kind: "verifyQuestionRubricVersion",
      id: "question-1",
      version: "2",
    }, "{}");
  });

  it("forwards MCQ options and answer key bodies verbatim through fixed operations", async () => {
    const questionBody = `{"syllabusId":"s","curriculumMapNodeId":"n","responseType":"multiple_choice","language":"en","prompt":"runtime","originType":"licensed_adaptation","contentSourceId":"source-1","sourceLocator":"metadata-ref","options":[{"id":"one","label":"runtime one"},{"id":"two","label":"runtime two"}]}`;
    mockedReadBody.mockResolvedValueOnce(questionBody);
    await create(new Request("http://localhost/x", { method: "POST", body: questionBody }));
    expect(mockedCallCore).toHaveBeenLastCalledWith({ kind: "createQuestion" }, questionBody);

    const rubricBody = `{"rubric":{"criteria":[{"id":"c1","marks":1}],"answerKey":{"correctOptionId":"one"}},"maxMarks":1}`;
    mockedReadBody.mockResolvedValueOnce(rubricBody);
    await createRubric(new Request("http://localhost/x", { method: "POST", body: rubricBody }), idParams);
    expect(mockedCallCore).toHaveBeenLastCalledWith({ kind: "createQuestionRubricVersion", id: "question-1" }, rubricBody);
  });

  it("returns safe proxy errors and stops after body rejection", async () => {
    mockedReadBody.mockRejectedValueOnce(new ProxyError(413, "payload_too_large", "request body is too large"));
    const bodyResponse = await create(new Request("http://localhost/x", { method: "POST", body: "{}" }));
    expect(bodyResponse.status).toBe(413);
    expect(mockedCallCore).not.toHaveBeenCalled();

    mockedCallCore.mockRejectedValueOnce(new ProxyError(400, "invalid_id", "identifier is not a valid resource id"));
    const idResponse = await get(new Request("http://localhost/x"), { params: Promise.resolve({ id: "../x" }) });
    expect(idResponse.status).toBe(400);
    await expect(idResponse.json()).resolves.toEqual({ error: "invalid_id", message: "identifier is not a valid resource id" });

    mockedCallCore.mockClear();
    mockedReadCanonicalBody.mockRejectedValueOnce(new ProxyError(400, "invalid_rubric_version", "request body must contain only a positive rubricVersion"));
    const canonicalResponse = await verify(new Request("http://localhost/x", { method: "POST", body: "{}" }), idParams);
    expect(canonicalResponse.status).toBe(400);
    expect(mockedCallCore).not.toHaveBeenCalled();
  });
});
