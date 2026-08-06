import { NextResponse } from "next/server";
import { callCoreLearner } from "@/lib/learner/core-proxy";
import { learnerErrorResponse } from "@/lib/learner/http";

interface RouteParams { params: Promise<{ id: string }> }

export async function POST(request: Request, { params }: RouteParams): Promise<NextResponse> {
  try {
    if ((await request.text()).trim() !== "") {
      return NextResponse.json({ error: "invalid_json", message: "request body must be empty" }, { status: 400 });
    }
    const { id } = await params;
    const result = await callCoreLearner({ kind: "createLearnerAttempt", questionId: id });
    return NextResponse.json(result.body, { status: result.status });
  } catch (err) {
    return learnerErrorResponse(err);
  }
}
