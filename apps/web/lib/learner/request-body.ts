import "server-only";
import { LearnerProxyError } from "./core-proxy";

const MAX_LEARNER_BODY_BYTES = 4096;

/** Reads at most Core's 4096-byte learner write limit without ever buffering beyond it. */
export async function readLearnerRequestBody(request: Request): Promise<string> {
  const declaredLength = request.headers.get("content-length");
  if (declaredLength !== null && /^\d+$/.test(declaredLength)) {
    if (BigInt(declaredLength) > BigInt(MAX_LEARNER_BODY_BYTES)) {
      throw new LearnerProxyError(413, "payload_too_large", "request body is too large");
    }
  }

  if (request.body === null) {
    return "";
  }

  const reader = request.body.getReader();
  const decoder = new TextDecoder();
  let bytesRead = 0;
  let text = "";
  while (true) {
    const { done, value } = await reader.read();
    if (done) {
      return text + decoder.decode();
    }
    bytesRead += value.byteLength;
    if (bytesRead > MAX_LEARNER_BODY_BYTES) {
      await reader.cancel().catch(() => undefined);
      throw new LearnerProxyError(413, "payload_too_large", "request body is too large");
    }
    text += decoder.decode(value, { stream: true });
  }
}
