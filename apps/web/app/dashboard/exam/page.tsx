import { requireEditorialRole } from "@/lib/editorial/role";
import { ExamScreen } from "./exam-screen";

export default async function ExamPage() {
  const role = await requireEditorialRole();
  return (
    <main id="main" style={{ padding: "var(--space-6)" }}>
      <ExamScreen role={role} />
    </main>
  );
}
