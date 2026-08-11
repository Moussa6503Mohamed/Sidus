import { expect, test, type Page } from "@playwright/test";
import { assertSynthetic, syntheticRun, syntheticStrings, type SyntheticQuestion } from "./lib/synthetic";

/**
 * The full local journey (T-0025), driven entirely through the real signed-in UI so that every
 * step exercises the same page → BFF route → Core path a human would:
 *
 *   pending synthetic source → approve → create node → verify node → create MCQ draft →
 *   create rubric version → verify rubric → choose canonical + verify question →
 *   learner Practice selection → submit → feedback visible
 *
 * Runs as the `admin` profile because that single role legitimately holds both the editing and
 * the reviewing permissions, and every recognized role holds `learner_question:read`. Nothing is
 * bypassed: Core re-checks authorization, the approved-source gate, and grounding on every write.
 *
 * All content is opaque nonce data generated at runtime (see lib/synthetic.ts) and asserted to be
 * so before the first keystroke. No educational, past-paper, mark-scheme, or otherwise
 * source-derived text is ever typed, and nothing is seeded — the records exist only in the
 * operator's local `sidus-local-import` database.
 */

const SYLLABUS_CODE = process.env.SIDUS_E2E_SYLLABUS_CODE?.trim() || "9700";

/** Reads the option `value` of a `<select>` whose visible text contains `text`. Used to turn a
 * human-readable syllabus label into the catalogue UUID the page actually posts, without the
 * harness hard-coding any environment-specific id. */
async function optionValueByText(page: Page, selectSelector: string, text: string): Promise<string> {
  const option = page.locator(`${selectSelector} option`).filter({ hasText: text }).first();
  await expect(
    option,
    `no option matching "${text}" in ${selectSelector} — is the catalogue syllabus active on this stack?`,
  ).toHaveCount(1);
  const value = await option.getAttribute("value");
  expect(value, `option "${text}" in ${selectSelector} has no value`).toBeTruthy();
  return value as string;
}

async function createAndVerifySecondQuestion(page: Page, question: SyntheticQuestion, nodeCode: string, nodeLabel: string) {
  await page.getByRole("button", { name: "New question" }).click();
  await page.locator("#question-origin").selectOption("original");
  await page.locator("#question-node").selectOption({ label: `${nodeCode} — ${nodeLabel}` });
  await page.locator("#question-response-type").selectOption("multiple_choice");
  await page.locator("#question-language").fill("en");
  await page.locator("#question-prompt").fill(question.prompt);
  for (const [index, option] of question.options.entries()) {
    await page.getByRole("button", { name: "Add option" }).click();
    await page.locator(`#question-option-id-${index}`).fill(option.id);
    await page.locator(`#question-option-label-${index}`).fill(option.label);
  }
  await page.getByRole("button", { name: "Create draft" }).click();
  await expect(page.getByRole("heading", { name: "Rubric versions" })).toBeVisible();

  const [correct, incorrect] = question.options;
  await page.locator("#rubric-max-marks").fill("1");
  await page.locator("#rubric-correct-option").selectOption(correct.id);
  await page.locator("#rubric-correct-explanation").fill(question.correctExplanation);
  await page.getByLabel(`Why option ${incorrect.id} is wrong (required)`).fill(question.incorrectExplanation);
  await page.getByRole("button", { name: "Add criterion" }).click();
  await page.locator("#criterion-id-0").fill(question.criterionId);
  await page.locator("#criterion-marks-0").fill("1");
  await page.getByRole("button", { name: "Create rubric version" }).click();
  await page.getByRole("button", { name: "Verify rubric" }).click();
  await page.getByRole("button", { name: "Confirm rubric verification" }).click();
  await page.getByLabel("Canonical rubric version").selectOption({ label: "Version 1" });
  await page.getByRole("button", { name: "Verify question" }).click();
  await page.getByRole("button", { name: "Confirm verification" }).click();
  await expect(page.getByText("This question is verified and cannot be edited.")).toBeVisible();
}

test("editorial source to learner practice feedback, end to end", async ({ page }) => {
  // A full ten-step journey through a dev-mode Next.js server; each navigation may compile a
  // route on first hit.
  test.setTimeout(5 * 60 * 1000);

  const run = syntheticRun();
  assertSynthetic(syntheticStrings(run));

  let syllabusId = "";
  let nodeId = "";

  await test.step("register a pending synthetic content source", async () => {
    await page.goto("/dashboard/editorial/sources");
    await expect(page.getByRole("heading", { name: "Content sources", level: 1 })).toBeVisible();

    await page.getByRole("button", { name: "New source" }).click();
    await page.locator("#source-title").fill(run.source.title);
    await page.locator("#source-url").fill(run.source.sourceUrl);
    await page.locator("#source-owner").fill(run.source.owner);
    await page.locator("#source-hash").fill(run.source.sourceHash);
    await page.locator("#source-licence").fill(run.source.licenceReference);
    await page.locator("#source-permitted-use").fill(run.source.permittedUse);
    await page.locator("#source-audience").fill(run.source.allowedAudience);
    await page.locator("#source-syllabus").selectOption(SYLLABUS_CODE);
    await page.getByRole("button", { name: "Create source" }).click();

    // Creating switches the panel to the new source's edit view — proof Core accepted the write.
    await expect(page.getByRole("heading", { name: `Edit ${run.source.title}` })).toBeVisible();
  });

  await test.step("approve the source as reviewer", async () => {
    await page.getByRole("button", { name: "Approve", exact: true }).click();
    await page.getByRole("button", { name: "Confirm approval" }).click();
    await expect(page.getByText("This source is already approved")).toBeVisible();
  });

  await test.step("create a draft curriculum-map node against that source", async () => {
    await page.goto("/dashboard/editorial/curriculum");
    await expect(page.getByRole("heading", { name: "Curriculum map", level: 1 })).toBeVisible();

    syllabusId = await optionValueByText(page, "#syllabus-picker", `(${SYLLABUS_CODE})`);
    await page.locator("#syllabus-picker").selectOption(syllabusId);

    await page.getByRole("button", { name: "New node" }).click();
    await page.locator("#node-kind").selectOption("topic");
    await page.locator("#node-code").fill(run.node.nodeCode);
    await page.locator("#node-label").fill(run.node.label);
    await page.locator("#node-source").selectOption({ label: run.source.title });
    await page.locator("#node-locator").fill(run.node.sourceLocator);
    await page.getByRole("button", { name: "Create node" }).click();

    await expect(page.getByRole("heading", { name: `Edit ${run.node.nodeCode}` })).toBeVisible();
  });

  await test.step("verify the node as reviewer", async () => {
    await page.getByRole("button", { name: "Verify", exact: true }).click();
    await page.getByRole("button", { name: "Confirm verification" }).click();
    await expect(page.getByText("This node is verified and can no longer be edited.")).toBeVisible();
  });

  await test.step("create a multiple-choice question draft grounded in that node", async () => {
    await page.goto("/dashboard/editorial/questions");
    await expect(page.getByRole("heading", { name: "Editorial questions", level: 1 })).toBeVisible();

    // Do not let a failed workspace load consume this journey's five-minute overall budget.
    // A visible picker proves both prerequisite BFF reads completed; any failure now points at
    // the question workspace boundary within seconds.
    await expect(page.locator("#question-syllabus")).toBeVisible({ timeout: 15_000 });
    await page.locator("#question-syllabus").selectOption(syllabusId);
    await page.getByRole("button", { name: "New question" }).click();

    await page.locator("#question-origin").selectOption("original");
    await page
      .locator("#question-node")
      .selectOption({ label: `${run.node.nodeCode} — ${run.node.label}` });
    nodeId = await page.locator("#question-node").inputValue();
    await page.locator("#question-response-type").selectOption("multiple_choice");
    await page.locator("#question-language").fill("en");
    await page.locator("#question-prompt").fill(run.question.prompt);

    for (const [index, option] of run.question.options.entries()) {
      await page.getByRole("button", { name: "Add option" }).click();
      await page.locator(`#question-option-id-${index}`).fill(option.id);
      await page.locator(`#question-option-label-${index}`).fill(option.label);
    }

    await page.getByRole("button", { name: "Create draft" }).click();
    await expect(page.getByRole("heading", { name: "Rubric versions" })).toBeVisible();
  });

  await test.step("create a draft rubric version with complete MCQ feedback", async () => {
    const [correct, incorrect] = run.question.options;

    await page.locator("#rubric-max-marks").fill("1");
    await page.locator("#rubric-correct-option").selectOption(correct.id);
    await page.locator("#rubric-correct-explanation").fill(run.question.correctExplanation);
    await page
      .getByLabel(`Why option ${incorrect.id} is wrong (required)`)
      .fill(run.question.incorrectExplanation);

    await page.getByRole("button", { name: "Add criterion" }).click();
    await page.locator("#criterion-id-0").fill(run.question.criterionId);
    await page.locator("#criterion-marks-0").fill("1");

    await page.getByRole("button", { name: "Create rubric version" }).click();
    await expect(page.getByRole("cell", { name: "Current revision" })).toBeVisible();
  });

  await test.step("verify the rubric version as reviewer", async () => {
    await page.getByRole("button", { name: "Verify rubric" }).click();
    await page.getByRole("button", { name: "Confirm rubric verification" }).click();
    await expect(page.getByRole("button", { name: "Verify rubric" })).toHaveCount(0);
  });

  await test.step("choose the canonical rubric and verify the question", async () => {
    await page.getByLabel("Canonical rubric version").selectOption({ label: "Version 1" });
    await page.getByRole("button", { name: "Verify question" }).click();
    await page.getByRole("button", { name: "Confirm verification" }).click();

    await expect(page.getByText("Canonical rubric selected. Replacement is not allowed.")).toBeVisible();
    await expect(page.getByText("This question is verified and cannot be edited.")).toBeVisible();
  });

  await test.step("create and verify a second opaque MCQ for complete Exam Mode flow", async () => {
    await createAndVerifySecondQuestion(page, run.secondQuestion, run.node.nodeCode, run.node.label);
  });

  await test.step("select the now-eligible question in learner Practice", async () => {
    await page.goto("/dashboard/practice");
    await expect(page.getByRole("heading", { name: "Practice", level: 1 })).toBeVisible();

    await page.locator("#practice-syllabus-id").selectOption(syllabusId);
    await page.getByRole("button", { name: "Load questions" }).click();

    await expect(page.getByText(run.question.prompt, { exact: true })).toBeVisible();
  });

  await test.step("submit an answer and read the canonical feedback", async () => {
    const [correct, incorrect] = run.question.options;
    const card = page.getByRole("listitem").filter({
      has: page.getByText(run.question.prompt, { exact: true }),
    });
    await expect(card).toHaveCount(1);

    await card.getByRole("radio", { name: correct.label }).click();
    await card.getByRole("button", { name: "Submit answer" }).click();

    const feedback = card.getByRole("region", { name: "Answer feedback" });
    await expect(feedback).toBeVisible();
    await expect(feedback.getByRole("heading", { name: "Correct", exact: true })).toBeVisible();
    await expect(feedback).toContainText("1 / 1");
    await expect(feedback).toContainText(run.question.correctExplanation);
    await expect(feedback).toContainText(run.question.incorrectExplanation);
    await expect(feedback).toContainText(incorrect.label);

    // One-shot attempts: the submit control is gone once marked, never re-armed.
    await expect(card.getByRole("button", { name: "Submit answer" })).toHaveCount(0);
  });

  await test.step("Exam Mode completes a two-question paper through learner-safe marking", async () => {
    await page.goto("/dashboard/exam");
    await expect(page.getByRole("heading", { name: "Exam Mode", level: 1 })).toBeVisible();
    await page.locator("#exam-syllabus").selectOption(syllabusId);
    await page.locator("#exam-node").fill(nodeId);
    await page.getByRole("button", { name: "Start exam" }).click();
    await expect(page.getByText(run.question.prompt)).toBeVisible();
    await page.getByRole("radio", { name: run.question.options[0].label }).click();
    await page.getByRole("button", { name: "Next", exact: true }).click();
    await expect(page.getByText(run.secondQuestion.prompt)).toBeVisible();
    await page.getByRole("radio", { name: run.secondQuestion.options[0].label }).click();
    await page.getByRole("button", { name: "Submit all" }).click();
    await page.getByRole("button", { name: "Confirm submit" }).click();
    await expect(page.getByRole("heading", { name: "Exam results", level: 1 })).toBeVisible();
    await expect(page.getByText(/answered score: 2 \/ 2/i)).toBeVisible();
  });
});
