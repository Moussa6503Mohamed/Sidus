import { requireEditorialRole } from "@/lib/editorial/role";
import { PrivateUploadsScreen } from "./screen";
export default async function PrivateUploadsPage(){ const role=await requireEditorialRole(); return <main id="main" style={{padding:"var(--space-6)"}}><PrivateUploadsScreen role={role}/></main>; }
