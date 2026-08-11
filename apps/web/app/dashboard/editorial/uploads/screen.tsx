"use client";
import { useEffect, useState } from "react";
import type { EditorialRole } from "@/lib/editorial/permissions";
type Upload={id:string;originalFilename:string;byteSize:number;status:string;retentionState:string;createdAt:string;};
type Source={id:string;title:string;status:string;catalogueSyllabusId:string|null};
const api=async(path:string,init?:RequestInit)=>{const r=await fetch(path,init);const b=await r.json().catch(()=>({}));if(!r.ok)throw new Error(typeof b.message==="string"?b.message:"request failed");return b;};
export function PrivateUploadsScreen({role}:{role:EditorialRole}){
 const [items,setItems]=useState<Upload[]|null>(null),[sources,setSources]=useState<Source[]>([]),[file,setFile]=useState<File|null>(null),[selected,setSelected]=useState<Record<string,string>>({}),[message,setMessage]=useState<string|null>(null),[busy,setBusy]=useState(false);
 const load=async()=>{try{const [u,s]=await Promise.all([api("/api/editorial/private-uploads"),api("/api/editorial/content-sources?status=approved")]);setItems(u.items??[]);setSources((s.items??[]).filter((x:Source)=>x.catalogueSyllabusId));}catch(e){setMessage(e instanceof Error?e.message:"Could not load uploads");}};
 useEffect(()=>{if(role==="admin")void load();},[role]);
 if(role!=="admin")return <section><h1>Private intake</h1><p role="alert">Admin access required.</p></section>;
 const mutate=async(fn:()=>Promise<unknown>)=>{setBusy(true);setMessage(null);try{await fn();await load();setMessage("Updated.");}catch(e){setMessage(e instanceof Error?e.message:"Request failed");}finally{setBusy(false);}};
 return <section style={{maxWidth:"72rem"}}><h1>Private PDF intake</h1><p>Files enter private quarantine only. No learner access. Review remains disabled until scan and future Sonnet evaluation are approved.</p>
 {message&&<p role="status">{message}</p>}
 <form onSubmit={e=>{e.preventDefault();if(!file){setMessage("Select a PDF.");return;}void mutate(async()=>{const fd=new FormData();fd.set("file",file);await api("/api/editorial/private-uploads",{method:"POST",body:fd});setFile(null);});}}>
 <label>PDF <input aria-label="PDF file" type="file" accept="application/pdf,.pdf" onChange={e=>setFile(e.target.files?.[0]??null)}/></label><button disabled={busy} type="submit">Upload to quarantine</button>
 </form>
 {items===null?<p>Loading uploads…</p>:items.length===0?<p>No private uploads.</p>:<table><thead><tr><th>File</th><th>Size</th><th>Status</th><th>Actions</th></tr></thead><tbody>{items.map(x=><tr key={x.id}><td>{x.originalFilename}</td><td>{Math.ceil(x.byteSize/1024)} KiB</td><td>{x.status}</td><td>{x.status==="quarantined"&&<span>Awaiting scanner attestation</span>}{x.status==="scan_clean"&&<><select aria-label={`Source for ${x.originalFilename}`} value={selected[x.id]??""} onChange={e=>setSelected({...selected,[x.id]:e.target.value})}><option value="">Choose approved source</option>{sources.map(s=><option key={s.id} value={s.id}>{s.title}</option>)}</select><button disabled={busy||!selected[x.id]} onClick={()=>void mutate(()=>api(`/api/editorial/private-uploads/${x.id}/review-jobs`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({contentSourceId:selected[x.id]})}))}>Queue review</button></>}{x.status!=="deletion_requested"&&<button disabled={busy} onClick={()=>void mutate(()=>api(`/api/editorial/private-uploads/${x.id}/deletion-request`,{method:"POST"}))}>Request deletion</button>}</td></tr>)}</tbody></table>}
 </section>;
}
