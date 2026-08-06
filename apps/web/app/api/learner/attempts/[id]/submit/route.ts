import { NextResponse } from "next/server";
import { callCoreLearner } from "@/lib/learner/core-proxy";
import { learnerErrorResponse } from "@/lib/learner/http";
import { readLearnerRequestBody } from "@/lib/learner/request-body";

interface RouteParams { params: Promise<{ id: string }> }

const exactSubmitBody = /^\s*\{\s*"selectedOptionId"\s*:\s*("(?:\\(?:["\\/bfnrt]|u[0-9a-fA-F]{4})|[^"\\\u0000-\u001F])*")\s*\}\s*$/;

export async function POST(request: Request, { params }: RouteParams): Promise<NextResponse> {
  try {
    const match = exactSubmitBody.exec(await readLearnerRequestBody(request));
    if (!match) {
      return NextResponse.json({ error: "invalid_json", message: "request body is invalid" }, { status: 400 });
    }
    const selectedOptionId = (JSON.parse(match[1]) as string).trim();
    if (!selectedOptionId || Array.from(selectedOptionId).length > 64) {
      return NextResponse.json({ error: "invalid_json", message: "request body is invalid" }, { status: 400 });
    }
    const { id } = await params;
    const result = await callCoreLearner({
      kind: "submitLearnerAttempt",
      attemptId: id,
      selectedOptionId,
    });
    return NextResponse.json(result.body, { status: result.status });
  } catch (err) {
    return learnerErrorResponse(err);
  }
}
