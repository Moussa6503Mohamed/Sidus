import { NextResponse } from "next/server";
import { callCoreLearner } from "@/lib/learner/core-proxy";
import { learnerErrorResponse } from "@/lib/learner/http";

export async function GET(request: Request): Promise<NextResponse> {
  try {
    const url = new URL(request.url);
    const syllabusId = url.searchParams.get("syllabusId");
    if (!syllabusId) {
      return NextResponse.json(
        { error: "missing_required_fields", missing: ["syllabusId"] },
        { status: 400 },
      );
    }
    const curriculumMapNodeId = url.searchParams.get("curriculumMapNodeId") ?? undefined;
    const result = await callCoreLearner({ kind: "listLearnerQuestions", syllabusId, curriculumMapNodeId });
    return NextResponse.json(result.body, { status: result.status });
  } catch (err) {
    return learnerErrorResponse(err);
  }
}
