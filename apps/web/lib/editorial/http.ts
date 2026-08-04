import { NextResponse } from "next/server";
import { ProxyError } from "./core-proxy";

/** Maps a callCore()/readSafeJsonBody() failure to a safe JSON response. Never forwards raw
 * error text from an exception that is not a ProxyError (which only ever carries static,
 * pre-written messages — never secrets, tokens, or upstream driver text). */
export function errorResponse(err: unknown): NextResponse {
  if (err instanceof ProxyError) {
    return NextResponse.json({ error: err.code, message: err.message }, { status: err.status });
  }
  return NextResponse.json(
    { error: "internal_error", message: "an unexpected error occurred" },
    { status: 500 },
  );
}
