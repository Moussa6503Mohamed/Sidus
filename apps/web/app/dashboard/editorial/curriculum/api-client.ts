import type {
  ContentSource,
  CreateCurriculumMapNodeRequest,
  CurriculumMapNode,
  Syllabus,
  UpdateCurriculumMapNodeRequest,
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
  if (init?.body !== undefined) {
    headers["Content-Type"] = "application/json";
  }

  const response = await fetch(path, { ...init, headers });
  const text = await response.text();
  const body: unknown = text ? JSON.parse(text) : null;

  if (!response.ok) {
    const errorBody = body as { error?: unknown; message?: unknown } | null;
    const code = typeof errorBody?.error === "string" ? errorBody.error : "request_failed";
    const message =
      typeof errorBody?.message === "string" ? errorBody.message : "The request could not be completed.";
    throw new ApiError(response.status, code, message);
  }

  return body as T;
}

export function listSyllabuses(): Promise<ListResponse<Syllabus>> {
  return request("/api/editorial/syllabuses");
}

/** Approved content sources only — the set a node's contentSourceId selector may pick from. */
export function listApprovedContentSources(): Promise<ListResponse<ContentSource>> {
  return request("/api/editorial/content-sources?status=approved");
}

export function listCurriculumMapNodes(syllabusId: string, status?: string): Promise<ListResponse<CurriculumMapNode>> {
  const query = new URLSearchParams({ syllabusId });
  if (status) query.set("status", status);
  return request(`/api/editorial/curriculum-map/nodes?${query.toString()}`);
}

export function createCurriculumMapNode(input: CreateCurriculumMapNodeRequest): Promise<CurriculumMapNode> {
  return request("/api/editorial/curriculum-map/nodes", { method: "POST", body: JSON.stringify(input) });
}

export function updateCurriculumMapNode(
  id: string,
  input: UpdateCurriculumMapNodeRequest,
): Promise<CurriculumMapNode> {
  return request(`/api/editorial/curriculum-map/nodes/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}

export function verifyCurriculumMapNode(id: string): Promise<CurriculumMapNode> {
  return request(`/api/editorial/curriculum-map/nodes/${encodeURIComponent(id)}/verify`, {
    method: "POST",
    body: "{}",
  });
}

export function retireCurriculumMapNode(id: string): Promise<CurriculumMapNode> {
  return request(`/api/editorial/curriculum-map/nodes/${encodeURIComponent(id)}/retire`, {
    method: "POST",
    body: "{}",
  });
}
