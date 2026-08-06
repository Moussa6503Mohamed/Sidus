import type {
  CreateQuestionRequest,
  CreateQuestionRubricVersionRequest,
  CurriculumMapNode,
  ContentSource,
  Question,
  QuestionRubricVersion,
  SelectCanonicalRubricRequest,
  Syllabus,
  UpdateQuestionRequest,
} from "./types";

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

interface ListResponse<T> {
  items: T[];
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const headers: Record<string, string> = { Accept: "application/json" };
  if (init?.body !== undefined) headers["Content-Type"] = "application/json";

  const response = await fetch(path, { ...init, headers });
  const text = await response.text();
  let body: unknown = null;
  if (text) {
    try {
      body = JSON.parse(text);
    } catch {
      throw new ApiError(502, "unexpected_response", "Editorial service returned an unexpected response.");
    }
  }
  if (!response.ok) {
    const errorBody = body as { error?: unknown; message?: unknown } | null;
    throw new ApiError(
      response.status,
      typeof errorBody?.error === "string" ? errorBody.error : "request_failed",
      typeof errorBody?.message === "string" ? errorBody.message : "Request could not be completed.",
    );
  }
  return body as T;
}

export function listSyllabuses(): Promise<ListResponse<Syllabus>> {
  return request("/api/editorial/syllabuses");
}

export function listApprovedContentSources(): Promise<ListResponse<ContentSource>> {
  return request("/api/editorial/content-sources?status=approved");
}

export function listVerifiedNodes(syllabusId: string): Promise<ListResponse<CurriculumMapNode>> {
  const query = new URLSearchParams({ syllabusId, status: "verified" });
  return request(`/api/editorial/curriculum-map/nodes?${query.toString()}`);
}

export function listQuestions(syllabusId: string, curriculumMapNodeId?: string): Promise<ListResponse<Question>> {
  const query = new URLSearchParams({ syllabusId, status: "all" });
  if (curriculumMapNodeId) query.set("curriculumMapNodeId", curriculumMapNodeId);
  return request(`/api/editorial/questions?${query.toString()}`);
}

export function createQuestion(input: CreateQuestionRequest): Promise<Question> {
  return request("/api/editorial/questions", { method: "POST", body: JSON.stringify(input) });
}

export function updateQuestion(id: string, input: UpdateQuestionRequest): Promise<Question> {
  return request(`/api/editorial/questions/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}

export function verifyQuestion(id: string, rubricVersion: number): Promise<Question> {
  const input: SelectCanonicalRubricRequest = { rubricVersion };
  return request(`/api/editorial/questions/${encodeURIComponent(id)}/verify`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function setCanonicalRubric(id: string, rubricVersion: number): Promise<Question> {
  const input: SelectCanonicalRubricRequest = { rubricVersion };
  return request(`/api/editorial/questions/${encodeURIComponent(id)}/canonical-rubric`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function retireQuestion(id: string): Promise<Question> {
  return request(`/api/editorial/questions/${encodeURIComponent(id)}/retire`, { method: "POST", body: "{}" });
}

export function listRubricVersions(id: string): Promise<ListResponse<QuestionRubricVersion>> {
  return request(`/api/editorial/questions/${encodeURIComponent(id)}/rubric-versions`);
}

export function createRubricVersion(
  id: string,
  input: CreateQuestionRubricVersionRequest,
): Promise<QuestionRubricVersion> {
  return request(`/api/editorial/questions/${encodeURIComponent(id)}/rubric-versions`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function verifyRubricVersion(id: string, version: number): Promise<QuestionRubricVersion> {
  return request(
    `/api/editorial/questions/${encodeURIComponent(id)}/rubric-versions/${version}/verify`,
    { method: "POST", body: "{}" },
  );
}

const ERROR_MESSAGES: Record<string, string> = {
  unknown_node: "Selected curriculum node no longer exists. Refresh and choose a verified node.",
  unverified_node: "Selected curriculum node is not verified. Choose a verified node.",
  mismatched_node: "Selected curriculum node belongs to another syllabus.",
  unknown_source: "Question grounding source no longer exists.",
  unapproved_source: "Question grounding source is no longer approved.",
  unlinked_source: "Question grounding source is not linked to its syllabus.",
  mismatched_source: "Question grounding source belongs to another syllabus.",
  incomplete_source_rights: "Licensed source rights metadata is incomplete. Human review must complete it before use.",
  invalid_origin_type: "Select a valid question origin.",
  invalid_provenance: "Licensed provenance is incomplete or conflicts with selected origin.",
  invalid_lifecycle_transition: "Action is not allowed for current question or rubric lifecycle state.",
  missing_verified_rubric: "Verify at least one rubric version before verifying this question.",
  missing_current_verified_rubric: "Question content changed. Create and verify a rubric for current revision.",
  invalid_canonical_rubric: "Selected rubric does not belong to this question.",
  canonical_rubric_not_verified: "Selected rubric is not verified.",
  canonical_rubric_not_current: "Selected rubric was reviewed against an older question revision.",
  canonical_rubric_already_set: "Canonical rubric is already set and cannot be replaced.",
  invalid_rubric: "Rubric criteria are invalid. Check ids, marks, descriptors, and structure.",
  invalid_max_marks: "Maximum marks must be positive and equal sum of criterion marks.",
  duplicate_rubric_version: "Rubric version could not be allocated because of a duplicate. Retry.",
  no_changes: "No question fields changed.",
  no_updatable_fields: "Change at least one question field before saving.",
};

export function apiErrorMessage(err: unknown): string {
  if (err instanceof ApiError) return ERROR_MESSAGES[err.code] ?? err.message;
  return "Something went wrong. Try again.";
}
