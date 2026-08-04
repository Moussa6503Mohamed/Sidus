import { NextResponse } from "next/server";
import { callCore } from "@/lib/editorial/core-proxy";
import { errorResponse } from "@/lib/editorial/http";

export async function GET(): Promise<NextResponse> {
  try {
    const result = await callCore({ kind: "listSyllabuses" });
    return NextResponse.json(result.body, { status: result.status });
  } catch (err) {
    return errorResponse(err);
  }
}
