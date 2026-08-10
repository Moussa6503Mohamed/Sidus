import { expect, test } from "@playwright/test";

/**
 * Negative coverage for a signed-in user with no recognized `sidus_role` (T-0025).
 *
 * This is the deny-by-default case: an authenticated account that was never granted a role must
 * reach nothing — not the editorial workspaces, and not Practice either, even though Practice is
 * open to every *recognized* role. A missing or unparseable claim resolves to "unknown" on both
 * sides (web `parseSidusRole`, Core `ParseRole`); this proves neither side quietly widens it.
 */

const EDITORIAL_PAGES = [
  "/dashboard/editorial/sources",
  "/dashboard/editorial/curriculum",
  "/dashboard/editorial/questions",
];

test("an unknown role is refused every editorial workspace", async ({ page }) => {
  for (const path of EDITORIAL_PAGES) {
    await page.goto(path);
    await expect(page.getByRole("heading", { name: "No editorial access", level: 1 })).toBeVisible();
  }
});

test("an unknown role is refused Practice", async ({ page }) => {
  await page.goto("/dashboard/practice");
  await expect(page.getByRole("heading", { name: "No practice access", level: 1 })).toBeVisible();
});

test("an unknown role is refused both BFFs at Core, not merely in the UI", async ({ page }) => {
  await page.goto("/dashboard");

  const statuses = await page.evaluate(async () => {
    const [editorial, learner] = await Promise.all([
      fetch("/api/editorial/content-sources", { headers: { Accept: "application/json" } }),
      fetch("/api/learner/syllabuses", { headers: { Accept: "application/json" } }),
    ]);
    return { editorial: editorial.status, learner: learner.status };
  });
  expect([401, 403], "the BFF/Core boundary must refuse an unknown role's editorial read").toContain(
    statuses.editorial,
  );
  expect([401, 403], "the BFF/Core boundary must refuse an unknown role's learner read").toContain(
    statuses.learner,
  );
});
