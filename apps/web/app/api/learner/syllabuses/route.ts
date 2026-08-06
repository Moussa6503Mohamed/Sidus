import { NextResponse } from "next/server";
import { callCoreLearner } from "@/lib/learner/core-proxy";
import { learnerErrorResponse } from "@/lib/learner/http";

export async function GET(): Promise<NextResponse> {
  try {
    const result = await callCoreLearner({ kind: "listLearnerSyllabuses" });
    return NextResponse.json(result.body, { status: result.status });
  } catch (err) {
    return learnerErrorResponse(err);
  }
}
