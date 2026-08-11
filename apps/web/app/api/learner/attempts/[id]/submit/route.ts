import { NextResponse } from "next/server";
import { callCoreLearner } from "@/lib/learner/core-proxy";
import { learnerErrorResponse } from "@/lib/learner/http";
import { readLearnerRequestBody } from "@/lib/learner/request-body";

interface RouteParams { params: Promise<{ id: string }> }

function parseAnswer(raw: string): { selectedOptionId?: string; selectedOptionIds?: string[]; text?: string; value?: number } | null {
  try {
	const decodedKeys = Array.from(raw.matchAll(/"((?:\\.|[^"\\])*)"\s*:/g), (match) => JSON.parse(`"${match[1]}"`) as unknown);
	if (decodedKeys.some((key) => typeof key !== "string") || new Set(decodedKeys).size !== decodedKeys.length) return null;
    const value: unknown = JSON.parse(raw);
    if (!value || typeof value !== "object" || Array.isArray(value)) return null;
    const entries = Object.entries(value as Record<string, unknown>);
    if (entries.length !== 1) return null;
    const [key, answer] = entries[0];
    if (key === "selectedOptionId" && typeof answer === "string" && answer.trim() && Array.from(answer.trim()).length <= 64) return { selectedOptionId: answer.trim() };
    if (key === "selectedOptionIds" && Array.isArray(answer) && answer.length > 0 && answer.length <= 6 && answer.every((id) => typeof id === "string" && id.trim() && Array.from(id.trim()).length <= 64) && new Set(answer).size === answer.length) return { selectedOptionIds: answer.map((id) => (id as string).trim()) };
    if (key === "text" && typeof answer === "string" && answer.trim() && Array.from(answer.trim()).length <= 5000) return { text: answer.trim() };
    if (key === "value" && typeof answer === "number" && Number.isFinite(answer)) return { value: answer };
  } catch { /* safe invalid_json below */ }
  return null;
}

export async function POST(request: Request, { params }: RouteParams): Promise<NextResponse> {
  try {
    const answer = parseAnswer(await readLearnerRequestBody(request));
    if (!answer) {
      return NextResponse.json({ error: "invalid_json", message: "request body is invalid" }, { status: 400 });
    }
    const { id } = await params;
	const result = await callCoreLearner(answer.selectedOptionId ? { kind: "submitLearnerAttempt", attemptId: id, selectedOptionId: answer.selectedOptionId } : { kind: "submitLearnerAttempt", attemptId: id, answer });
    return NextResponse.json(result.body, { status: result.status });
  } catch (err) {
    return learnerErrorResponse(err);
  }
}
