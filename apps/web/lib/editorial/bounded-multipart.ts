import { ProxyError } from "./core-proxy";

export const MAX_PRIVATE_PDF_BYTES = 25 * 1024 * 1024;
// Multipart headers/boundaries need headroom. Body is still read through hard 25 MiB file check.
const MAX_MULTIPART_BYTES = MAX_PRIVATE_PDF_BYTES + 128 * 1024;

/** Reads multipart once with a hard stream cap before FormData parsing. A declared oversize or
 * unknown-length request is rejected before parser allocation/Core auth/network work. */
export async function readBoundedPDFForm(request: Request): Promise<FormData> {
  const contentType = request.headers.get("content-type") ?? "";
  if (!contentType.toLowerCase().startsWith("multipart/form-data;")) throw new ProxyError(400, "invalid_content_type", "upload must be multipart/form-data");
  const rawLength = request.headers.get("content-length");
  if (!rawLength || !/^[1-9][0-9]*$/.test(rawLength)) throw new ProxyError(411, "length_required", "upload content length is required");
  const declared = Number(rawLength);
  if (!Number.isSafeInteger(declared) || declared > MAX_MULTIPART_BYTES) throw new ProxyError(413, "payload_too_large", "PDF must be 25 MiB or smaller");
  if (!request.body) throw new ProxyError(400, "invalid_file", "select one PDF file");
  const reader = request.body.getReader(); const chunks: Uint8Array[]=[]; let size=0;
  while(true){const {done,value}=await reader.read();if(done)break;if(!value)continue;size+=value.byteLength;if(size>MAX_MULTIPART_BYTES){await reader.cancel().catch(()=>undefined);throw new ProxyError(413,"payload_too_large","PDF must be 25 MiB or smaller");}chunks.push(value)}
  const body = new Uint8Array(size); let offset=0; for(const chunk of chunks){body.set(chunk,offset);offset+=chunk.byteLength}
  return new Request("http://sidus.local/upload",{method:"POST",headers:{"Content-Type":contentType},body}).formData();
}
