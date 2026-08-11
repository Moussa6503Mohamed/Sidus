import { describe, expect, it } from "vitest";
import { readBoundedPDFForm } from "./bounded-multipart";

function multipart(length: string | null, body = "--x--\r\n") { return new Request("http://x", { method:"POST", headers:{"Content-Type":"multipart/form-data; boundary=x", ...(length ? {"Content-Length":length}: {})}, body }); }
describe("readBoundedPDFForm",()=>{
 it("rejects missing content length before parsing",async()=>{await expect(readBoundedPDFForm(multipart(null))).rejects.toMatchObject({status:411});});
 it("rejects declared oversize before parsing",async()=>{await expect(readBoundedPDFForm(multipart(String(25*1024*1024+128*1024+1)))).rejects.toMatchObject({status:413});});
 it("rejects non multipart",async()=>{const r=new Request("http://x",{method:"POST",headers:{"Content-Type":"application/pdf","Content-Length":"5"},body:"%PDF-"});await expect(readBoundedPDFForm(r)).rejects.toMatchObject({status:400});});
});
