import { requireEditorialRole } from "@/lib/editorial/role";
import { CurriculumMapScreen } from "./curriculum-screen";

export default async function CurriculumMapPage() {
  const role = await requireEditorialRole();

  return (
    <main id="main" style={{ padding: "var(--space-6)" }}>
      <CurriculumMapScreen role={role} />
    </main>
  );
}
