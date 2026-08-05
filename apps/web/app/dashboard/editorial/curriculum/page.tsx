import { requireEditorialRole } from "@/lib/editorial/role";
import { CurriculumMapScreen } from "./curriculum-screen";

export default async function CurriculumMapPage() {
  const role = await requireEditorialRole();

  return (
    <main style={{ padding: "1.5rem" }}>
      <CurriculumMapScreen role={role} />
    </main>
  );
}
