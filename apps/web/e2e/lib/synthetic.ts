import { randomBytes, randomUUID } from "node:crypto";

/**
 * Runtime-only synthetic fixture data for the E2E harness (T-0025).
 *
 * Every record the harness creates goes through the real editorial write path into the local
 * Core database, so its content is bound by the same non-negotiable rule as production content:
 * no past-paper wording, no mark-scheme wording, no book/PDF extract, no lightly rewritten
 * question, not even a plausible-looking Biology sentence. The only way to guarantee that is to
 * make the content carry no meaning at all: every string is the fixed harness marker plus an
 * opaque per-run nonce, and `assertSynthetic` proves it before anything is typed into a form.
 *
 * Nothing here is seeded, committed, or reused between runs — a fresh nonce per run also keeps
 * the unique `sourceUrl` constraint satisfied on repeat runs.
 */

/** Marker every generated value starts with, so a stray row is instantly identifiable as harness
 * output rather than editorial content. */
export const SYNTHETIC_MARKER = "SIDUS-E2E-SYNTHETIC";

/** A generated value is exactly the marker, the run nonce, and an ASCII field discriminator. */
const SYNTHETIC_VALUE_PATTERN = /^SIDUS-E2E-SYNTHETIC-[0-9a-f]{16}(-[a-z0-9]+)*$/;

export interface SyntheticOption {
  id: string;
  label: string;
}

export interface SyntheticSource {
  title: string;
  sourceUrl: string;
  owner: string;
  sourceHash: string;
  licenceReference: string;
  permittedUse: string;
  allowedAudience: string;
}

export interface SyntheticNode {
  nodeCode: string;
  label: string;
  sourceLocator: string;
}

export interface SyntheticQuestion {
  prompt: string;
  options: [SyntheticOption, SyntheticOption];
  correctExplanation: string;
  incorrectExplanation: string;
  criterionId: string;
}

export interface SyntheticRun {
  nonce: string;
  source: SyntheticSource;
  node: SyntheticNode;
  question: SyntheticQuestion;
}

/** 16 hex characters of CSPRNG output — opaque, unguessable, and carrying no information. */
export function syntheticNonce(): string {
  return randomBytes(8).toString("hex");
}

function value(nonce: string, ...parts: string[]): string {
  return [SYNTHETIC_MARKER, nonce, ...parts].join("-");
}

/**
 * Throws if any supplied string is not a pure marker+nonce value. Called by the specs before the
 * first keystroke, so a future edit that slips real prose (or anything source-derived) into a
 * fixture fails loudly instead of writing it to the database.
 */
export function assertSynthetic(values: Record<string, string>): void {
  const offenders = Object.entries(values)
    .filter(([, item]) => !SYNTHETIC_VALUE_PATTERN.test(item))
    .map(([key]) => key);
  if (offenders.length > 0) {
    throw new Error(
      `E2E fixture field(s) ${offenders.join(", ")} are not opaque synthetic values. ` +
        `Harness fixtures must be "${SYNTHETIC_MARKER}-<nonce>-<field>" only — never educational, ` +
        `past-paper, mark-scheme, or any source-derived text.`,
    );
  }
}

/**
 * Builds one run's worth of fixtures. `sourceUrl` uses the reserved-by-RFC-2606 `.invalid` TLD:
 * it is metadata identity only, is never dereferenced by Core or the harness, and cannot
 * accidentally resolve to a real host.
 */
export function syntheticRun(nonce: string = syntheticNonce()): SyntheticRun {
  const source: SyntheticSource = {
    title: value(nonce, "source"),
    sourceUrl: `https://e2e.invalid/${value(nonce, "source")}`,
    owner: value(nonce, "owner"),
    // Core stores sourceHash as opaque text; a random 64-hex value keeps the familiar shape
    // without implying any real file was hashed — no file is read by this harness.
    sourceHash: randomUUID().replaceAll("-", "") + randomUUID().replaceAll("-", ""),
    licenceReference: value(nonce, "licence"),
    permittedUse: value(nonce, "permitted", "use"),
    allowedAudience: value(nonce, "audience"),
  };

  const node: SyntheticNode = {
    nodeCode: value(nonce, "node"),
    label: value(nonce, "node", "label"),
    sourceLocator: value(nonce, "locator"),
  };

  const question: SyntheticQuestion = {
    prompt: value(nonce, "prompt"),
    options: [
      { id: value(nonce, "opt", "a"), label: value(nonce, "opt", "a", "label") },
      { id: value(nonce, "opt", "b"), label: value(nonce, "opt", "b", "label") },
    ],
    correctExplanation: value(nonce, "correct", "explanation"),
    incorrectExplanation: value(nonce, "incorrect", "explanation"),
    criterionId: value(nonce, "criterion"),
  };

  return { nonce, source, node, question };
}

/** Every generated string in a run, keyed for `assertSynthetic`. `sourceUrl` and `sourceHash`
 * are excluded: the first is a URL wrapper around a checked value, the second is pure random
 * hex — neither can carry prose. */
export function syntheticStrings(run: SyntheticRun): Record<string, string> {
  return {
    "source.title": run.source.title,
    "source.owner": run.source.owner,
    "source.licenceReference": run.source.licenceReference,
    "source.permittedUse": run.source.permittedUse,
    "source.allowedAudience": run.source.allowedAudience,
    "node.nodeCode": run.node.nodeCode,
    "node.label": run.node.label,
    "node.sourceLocator": run.node.sourceLocator,
    "question.prompt": run.question.prompt,
    "question.options[0].id": run.question.options[0].id,
    "question.options[0].label": run.question.options[0].label,
    "question.options[1].id": run.question.options[1].id,
    "question.options[1].label": run.question.options[1].label,
    "question.correctExplanation": run.question.correctExplanation,
    "question.incorrectExplanation": run.question.incorrectExplanation,
    "question.criterionId": run.question.criterionId,
  };
}
