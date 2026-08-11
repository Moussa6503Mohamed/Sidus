import { NextResponse } from "next/server";
import { callUploadCore, ProxyError } from "@/lib/editorial/core-proxy";
import { errorResponse } from "@/lib/editorial/http";

const MAX_BYTES = 25 * 1024 * 1024;
export async function GET(): Promise<NextResponse> { try { const r = await callUploadCore({ kind: "listPrivateUploads" }); return NextResponse.json(r.body, { status: r.status }); } catch (e) { return errorResponse(e); } }
export async function POST(request: Request): Promise<NextResponse> {
  try {
    const form = await request.formData(); const file = form.get("file");
    if (!(file instanceof File)) throw new ProxyError(400, "invalid_file", "select one PDF file");
    if (file.type !== "application/pdf" || !file.name.toLowerCase().endsWith(".pdf")) throw new ProxyError(400, "invalid_file", "file must be a PDF");
    if (file.size === 0 || file.size > MAX_BYTES) throw new ProxyError(413, "payload_too_large", "PDF must be 25 MiB or smaller");
    const r = await callUploadCore({ kind: "createPrivateUpload", filename: file.name }, await file.arrayBuffer());
    return NextResponse.json(r.body, { status: r.status });
  } catch (e) { return errorResponse(e); }
}
