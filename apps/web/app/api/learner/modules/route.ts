import { NextResponse } from "next/server";
import { callCoreLearner } from "@/lib/learner/core-proxy";
import { learnerErrorResponse } from "@/lib/learner/http";

// Fixed learner-safe module discovery route. It accepts only a syllabus id and never exposes
// editorial curriculum-map/source fields.
export async function GET(request: Request): Promise<NextResponse> {
  try {
    const syllabusId = new URL(request.url).searchParams.get("syllabusId");
    if (!syllabusId) {
      return NextResponse.json(
        { error: "missing_required_fields", missing: ["syllabusId"] },
        { status: 400 },
      );
    }
    const result = await callCoreLearner({ kind: "listLearnerModules", syllabusId });
    return NextResponse.json(result.body, { status: result.status });
  } catch (err) {
    return learnerErrorResponse(err);
  }
}
