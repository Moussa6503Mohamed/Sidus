import { NextResponse } from "next/server";
import { LearnerProxyError } from "./core-proxy";

/** Maps a callCoreLearner() failure to a safe JSON response. Never forwards raw error text from
 * an exception that is not a LearnerProxyError (which only ever carries static, pre-written
 * messages — never secrets, tokens, or upstream driver text). */
export function learnerErrorResponse(err: unknown): NextResponse {
  if (err instanceof LearnerProxyError) {
    return NextResponse.json({ error: err.code, message: err.message }, { status: err.status });
  }
  return NextResponse.json(
    { error: "internal_error", message: "an unexpected error occurred" },
    { status: 500 },
  );
}
