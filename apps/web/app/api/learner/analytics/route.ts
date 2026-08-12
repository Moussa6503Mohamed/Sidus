import { NextResponse } from "next/server";
import { callCoreLearner } from "@/lib/learner/core-proxy";
import { learnerErrorResponse } from "@/lib/learner/http";

export async function GET(request: Request): Promise<NextResponse> {
  try {
	if (new URL(request.url).search) {
	  return NextResponse.json({ error: "invalid_query", message: "analytics does not accept query parameters" }, { status: 400 });
	}
    const result = await callCoreLearner({ kind: "getLearnerAnalytics" });
    return NextResponse.json(result.body, { status: result.status });
  } catch (error) {
    return learnerErrorResponse(error);
  }
}
