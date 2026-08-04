import { NextResponse } from "next/server";
import { callCore } from "@/lib/editorial/core-proxy";
import { errorResponse } from "@/lib/editorial/http";

interface RouteParams {
  params: Promise<{ id: string }>;
}

// Approval carries no client-suppliable fields (decisionDate defaults to now server-side); the
// request body is intentionally ignored.
export async function POST(_request: Request, { params }: RouteParams): Promise<NextResponse> {
  try {
    const { id } = await params;
    const result = await callCore({ kind: "approveContentSource", id }, "{}");
    return NextResponse.json(result.body, { status: result.status });
  } catch (err) {
    return errorResponse(err);
  }
}
