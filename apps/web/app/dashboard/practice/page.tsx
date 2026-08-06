import { requireEditorialRole } from "@/lib/editorial/role";
import { PracticeScreen } from "./practice-screen";

export default async function PracticePage() {
  const role = await requireEditorialRole();

  return (
    <main id="main" style={{ padding: "var(--space-6)" }}>
      <PracticeScreen role={role} />
    </main>
  );
}
