import { expect, test } from "@playwright/test";

/**
 * Negative coverage for a real signed-in learner (T-0025).
 *
 * Two distinct things are asserted, and the second is the one that matters: the UI role gate is
 * cosmetic by design (D-0006/D-0011 — it only decides which controls render), so a learner being
 * shown "No editorial access" proves nothing on its own. The BFF/Core assertions below prove the
 * request is actually refused at the authorization authority, with no UI involved.
 */

const EDITORIAL_PAGES = [
  "/dashboard/editorial/sources",
  "/dashboard/editorial/curriculum",
  "/dashboard/editorial/questions",
];

test("a learner is refused every editorial workspace", async ({ page }) => {
  for (const path of EDITORIAL_PAGES) {
    await page.goto(path);
    await expect(page.getByRole("heading", { name: "No editorial access", level: 1 })).toBeVisible();
  }
});

test("a learner is refused the editorial BFF regardless of what the UI renders", async ({ page }) => {
  // Run fetch in the rendered browser context. `page.request` is an API client with distinct
  // request semantics; this proves the real same-origin cookie-bound BFF call a learner could
  // make from DevTools or a malicious page script.
  await page.goto("/dashboard");

  const status = await page.evaluate(async () => {
    const response = await fetch("/api/editorial/content-sources", {
      headers: { Accept: "application/json" },
    });
    return response.status;
  });
  expect(
    [401, 403],
    "the BFF/Core boundary must refuse a learner's editorial read; a 200 here would mean the only gate is the UI",
  ).toContain(status);
});

test("a learner keeps access to Practice", async ({ page }) => {
  await page.goto("/dashboard/practice");
  await expect(page.getByRole("heading", { name: "Practice", level: 1 })).toBeVisible();
  await expect(page.getByRole("heading", { name: "No practice access" })).toHaveCount(0);
});
