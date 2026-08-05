import { NextResponse } from "next/server";
import { callCore, readSafeJsonBody } from "@/lib/editorial/core-proxy";
import { errorResponse } from "@/lib/editorial/http";

export async function GET(request: Request): Promise<NextResponse> {
  try {
    const url = new URL(request.url);
    const syllabusId = url.searchParams.get("syllabusId") ?? "";
    const status = url.searchParams.get("status") ?? undefined;
    const result = await callCore({ kind: "listCurriculumMapNodes", syllabusId, status });
    return NextResponse.json(result.body, { status: result.status });
  } catch (err) {
    return errorResponse(err);
  }
}

export async function POST(request: Request): Promise<NextResponse> {
  try {
    const rawBody = await readSafeJsonBody(request);
    const result = await callCore({ kind: "createCurriculumMapNode" }, rawBody);
    return NextResponse.json(result.body, { status: result.status });
  } catch (err) {
    return errorResponse(err);
  }
}
