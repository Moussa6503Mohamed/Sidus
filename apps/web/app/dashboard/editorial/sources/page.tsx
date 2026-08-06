import { requireEditorialRole } from "@/lib/editorial/role";
import { EditorialSourcesScreen } from "./editorial-sources-screen";

export default async function EditorialSourcesPage() {
  const role = await requireEditorialRole();

  return (
    <main id="main" style={{ padding: "var(--space-6)" }}>
      <EditorialSourcesScreen role={role} />
    </main>
  );
}
