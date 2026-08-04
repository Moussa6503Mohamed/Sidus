import path from "node:path";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "server-only": path.join(__dirname, "./test/stubs/server-only.ts"),
      "@/": `${path.join(__dirname, "./")}/`,
      "@sidus/shared": path.join(__dirname, "../../packages/shared/src/contracts.ts"),
    },
  },
  test: {
    // Default environment is jsdom for component tests. Node-only suites (lib/editorial,
    // app/api/editorial) opt out with a `// @vitest-environment node` docblock.
    environment: "jsdom",
    globals: true,
    setupFiles: ["./vitest.setup.ts"],
  },
});
