import {NextResponse} from "next/server";import{callTeacherCore,TeacherProxyError,validateTeacherBody}from "@/lib/teacher/core-proxy";
const error=(e:unknown)=>NextResponse.json({error:e instanceof TeacherProxyError?e.code:"upstream_unavailable"},{status:e instanceof TeacherProxyError?e.status:502});
export async function GET(){try{const x=await callTeacherCore({kind:"listClasses"});return NextResponse.json(x.body,{status:x.status})}catch(e){return error(e)}}
export async function POST(r:Request){try{const b=validateTeacherBody("class",await r.text());const x=await callTeacherCore({kind:"createClass",body:b});return NextResponse.json(x.body,{status:x.status})}catch(e){return error(e)}}
