import { NextResponse } from "next/server";
import { callCoreLearner } from "@/lib/learner/core-proxy";
import { learnerErrorResponse } from "@/lib/learner/http";
export async function GET(_: Request, { params }: { params: Promise<{ id: string }> }): Promise<NextResponse> { try { const {id}=await params; const result=await callCoreLearner({kind:"getAssessmentResult",id}); return NextResponse.json(result.body,{status:result.status}); }catch(err){return learnerErrorResponse(err);} }
