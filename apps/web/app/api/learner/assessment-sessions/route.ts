import { NextResponse } from "next/server";
import { callCoreLearner } from "@/lib/learner/core-proxy";
import { learnerErrorResponse } from "@/lib/learner/http";

export async function POST(request: Request): Promise<NextResponse> {
  try {
    const body: unknown = await request.json();
    if (!body || typeof body !== "object" || Array.isArray(body)) return NextResponse.json({ error: "invalid_json", message: "request body is invalid" }, { status: 400 });
    const b = body as Record<string, unknown>;
    const result = await callCoreLearner({ kind: "createAssessmentSession", body: { mode: b.mode as "practice" | "exam", syllabusId: b.syllabusId as string, curriculumMapNodeId: b.curriculumMapNodeId as string | undefined, questionCount: b.questionCount as number, durationSeconds: b.durationSeconds as number | undefined } });
    return NextResponse.json(result.body, { status: result.status });
  } catch (err) { return learnerErrorResponse(err); }
}
