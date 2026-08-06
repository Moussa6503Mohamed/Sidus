import { requireEditorialRole } from "@/lib/editorial/role";
import { PracticeScreen } from "./practice-screen";

export default async function PracticePage() {
  const role = await requireEditorialRole();

  return (
    <main style={{ padding: "1.5rem" }}>
      <PracticeScreen role={role} />
    </main>
  );
}
