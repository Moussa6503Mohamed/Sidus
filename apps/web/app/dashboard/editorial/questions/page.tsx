import { requireEditorialRole } from "@/lib/editorial/role";
import { EditorialQuestionsScreen } from "./editorial-questions-screen";

export default async function EditorialQuestionsPage() {
  const role = await requireEditorialRole();
  return (
    <main id="main" style={{ padding: "var(--space-6)" }}>
      <EditorialQuestionsScreen role={role} />
    </main>
  );
}
