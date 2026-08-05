import { NextResponse } from "next/server";
import { callCore } from "@/lib/editorial/core-proxy";
import { errorResponse } from "@/lib/editorial/http";

interface RouteParams {
  params: Promise<{ id: string; version: string }>;
}

export async function POST(_request: Request, { params }: RouteParams): Promise<NextResponse> {
  try {
    const { id, version } = await params;
    const result = await callCore({ kind: "verifyQuestionRubricVersion", id, version }, "{}");
    return NextResponse.json(result.body, { status: result.status });
  } catch (err) {
    return errorResponse(err);
  }
}
