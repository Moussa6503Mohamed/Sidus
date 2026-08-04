import type {
  ContentSource,
  CreateContentSourceRequest,
  Syllabus,
  UpdateContentSourceRequest,
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

export function listContentSources(status?: string): Promise<ListResponse<ContentSource>> {
  const query = status ? `?status=${encodeURIComponent(status)}` : "";
  return request(`/api/editorial/content-sources${query}`);
}

export function createContentSource(input: CreateContentSourceRequest): Promise<ContentSource> {
  return request("/api/editorial/content-sources", { method: "POST", body: JSON.stringify(input) });
}

export function updateContentSource(id: string, input: UpdateContentSourceRequest): Promise<ContentSource> {
  return request(`/api/editorial/content-sources/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}

export function approveContentSource(id: string): Promise<ContentSource> {
  return request(`/api/editorial/content-sources/${encodeURIComponent(id)}/approve`, {
    method: "POST",
    body: "{}",
  });
}

export function rejectContentSource(id: string, reason: string): Promise<ContentSource> {
  return request(`/api/editorial/content-sources/${encodeURIComponent(id)}/reject`, {
    method: "POST",
    body: JSON.stringify({ reason }),
  });
}

export function listSyllabuses(): Promise<ListResponse<Syllabus>> {
  return request("/api/editorial/syllabuses");
}
