import "server-only";
import { TeacherProxyError } from "./core-proxy";

export const MAX_TEACHER_BODY_BYTES = 8192;

/** Stream only the bounded teacher JSON body. On an untrusted/chunked overflow, cancel before
 * buffering the excess and fail before Clerk or Core is reached. */
export async function readTeacherRequestBody(request: Request): Promise<string> {
  const declared = request.headers.get("content-length");
  if (declared !== null && /^\d+$/.test(declared) && BigInt(declared) > BigInt(MAX_TEACHER_BODY_BYTES)) {
    throw new TeacherProxyError(413, "payload_too_large", "request body is too large");
  }
  if (request.body === null) return "";
  const reader = request.body.getReader(); const decoder = new TextDecoder(); let size = 0; let text = "";
  while (true) { const { done, value } = await reader.read(); if (done) return text + decoder.decode(); size += value.byteLength; if (size > MAX_TEACHER_BODY_BYTES) { await reader.cancel().catch(() => undefined); throw new TeacherProxyError(413, "payload_too_large", "request body is too large"); } text += decoder.decode(value, { stream: true }); }
}
