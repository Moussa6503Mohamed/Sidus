import { NextResponse } from "next/server";
import { callCoreLearner } from "@/lib/learner/core-proxy";
import { learnerErrorResponse } from "@/lib/learner/http";

interface RouteParams { params: Promise<{ id: string }> }
export async function POST(_request: Request, { params }: RouteParams): Promise<NextResponse> {
  try { const { id } = await params; const result = await callCoreLearner({ kind: "requestWrittenMarking", attemptId: id }); return NextResponse.json(result.body, { status: result.status }); }
  catch (err) { return learnerErrorResponse(err); }
}
export async function GET(_request: Request, { params }: RouteParams): Promise<NextResponse> {
  try { const { id } = await params; const result = await callCoreLearner({ kind: "getWrittenMarking", attemptId: id }); return NextResponse.json(result.body, { status: result.status }); }
  catch (err) { return learnerErrorResponse(err); }
}
