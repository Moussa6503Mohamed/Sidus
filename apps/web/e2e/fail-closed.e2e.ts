import { expect, test } from "@playwright/test";

/**
 * Fail-closed coverage (T-0025). This project deliberately carries **no** storage state, so it
 * needs no captured Clerk session and always runs.
 *
 * It answers the question the authenticated suites cannot: when the session is absent, forged, or
 * expired, does the application refuse — or does it quietly serve something? A harness that only
 * ever ran with a good session could pass while authentication was entirely broken.
 */

const PROTECTED_PAGES = [
  "/dashboard",
  "/dashboard/practice",
  "/dashboard/editorial/sources",
  "/dashboard/editorial/curriculum",
  "/dashboard/editorial/questions",
];

test("a signed-out browser is redirected away from every protected page", async ({ page }) => {
  for (const path of PROTECTED_PAGES) {
    await page.goto(path);
    await expect(page).toHaveURL(/sign-in/);
  }
});

test("a signed-out browser is refused by both BFFs", async ({ page }) => {
  const learner = await page.request.get("/api/learner/syllabuses", {
    headers: { Accept: "application/json" },
  });
  expect(learner.status(), "the learner BFF must refuse an unauthenticated caller").toBe(401);

  const editorial = await page.request.get("/api/editorial/content-sources", {
    headers: { Accept: "application/json" },
  });
  expect(editorial.status(), "the editorial BFF must refuse an unauthenticated caller").toBe(401);
});

test("a forged or expired session cookie is refused, not partially trusted", async ({ browser }) => {
  // Shaped like a Clerk session cookie but signed by nobody, with an expiry already in the past.
  // No real credential is involved — the point is that neither the cookie's *name* nor its mere
  // presence buys any trust.
  const context = await browser.newContext({
    storageState: {
      cookies: [
        {
          name: "__session",
          value: "forged-not-a-real-jwt",
          domain: "localhost",
          path: "/",
          expires: Math.floor(Date.now() / 1000) - 3600,
          httpOnly: true,
          secure: false,
          sameSite: "Lax",
        },
      ],
      origins: [],
    },
  });

  try {
    const page = await context.newPage();
    await page.goto("/dashboard/practice");
    await expect(page).toHaveURL(/sign-in/);
  } finally {
    await context.close();
  }
});
