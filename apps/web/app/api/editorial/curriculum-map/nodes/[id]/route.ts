import { NextResponse } from "next/server";
import { callCore, readSafeJsonBody } from "@/lib/editorial/core-proxy";
import { errorResponse } from "@/lib/editorial/http";

interface RouteParams {
  params: Promise<{ id: string }>;
}

export async function GET(_request: Request, { params }: RouteParams): Promise<NextResponse> {
  try {
    const { id } = await params;
    const result = await callCore({ kind: "getCurriculumMapNode", id });
    return NextResponse.json(result.body, { status: result.status });
  } catch (err) {
    return errorResponse(err);
  }
}

export async function PATCH(request: Request, { params }: RouteParams): Promise<NextResponse> {
  try {
    const { id } = await params;
    const rawBody = await readSafeJsonBody(request);
    const result = await callCore({ kind: "updateCurriculumMapNode", id }, rawBody);
    return NextResponse.json(result.body, { status: result.status });
  } catch (err) {
    return errorResponse(err);
  }
}
