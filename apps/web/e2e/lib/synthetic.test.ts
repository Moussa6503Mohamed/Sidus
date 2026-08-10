// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  assertSynthetic,
  SYNTHETIC_MARKER,
  syntheticNonce,
  syntheticRun,
  syntheticStrings,
} from "./synthetic";

describe("syntheticNonce", () => {
  it("produces 16 opaque hex characters", () => {
    expect(syntheticNonce()).toMatch(/^[0-9a-f]{16}$/);
  });

  it("never repeats across runs, so repeat runs cannot collide on unique sourceUrl", () => {
    const nonces = new Set(Array.from({ length: 200 }, () => syntheticNonce()));
    expect(nonces.size).toBe(200);
  });
});

describe("syntheticRun", () => {
  it("marks every generated string as harness output", () => {
    const run = syntheticRun();
    for (const value of Object.values(syntheticStrings(run))) {
      expect(value.startsWith(SYNTHETIC_MARKER)).toBe(true);
      expect(value).toContain(run.nonce);
    }
  });

  it("passes its own synthetic assertion", () => {
    expect(() => assertSynthetic(syntheticStrings(syntheticRun()))).not.toThrow();
  });

  it("keeps option ids inside Core's 64-character limit", () => {
    const run = syntheticRun();
    for (const option of run.question.options) {
      expect([...option.id].length).toBeLessThanOrEqual(64);
    }
  });

  it("gives the two options distinct ids and labels", () => {
    const [first, second] = syntheticRun().question.options;
    expect(first.id).not.toBe(second.id);
    expect(first.label).not.toBe(second.label);
  });

  it("uses a reserved .invalid host so the source URL can never resolve anywhere real", () => {
    expect(new URL(syntheticRun().source.sourceUrl).hostname.endsWith(".invalid")).toBe(true);
  });

  it("produces a 64-character hex source hash without hashing any file", () => {
    expect(syntheticRun().source.sourceHash).toMatch(/^[0-9a-f]{64}$/);
  });

  it("produces entirely different fixtures on each call", () => {
    const first = syntheticStrings(syntheticRun());
    const second = syntheticStrings(syntheticRun());
    for (const key of Object.keys(first)) {
      expect(first[key]).not.toBe(second[key]);
    }
  });
});

describe("assertSynthetic", () => {
  it("rejects prose that could carry educational or source-derived wording", () => {
    expect(() =>
      assertSynthetic({ "question.prompt": "Which organelle carries out photosynthesis?" }),
    ).toThrow(/question\.prompt/);
  });

  it("rejects a value that merely contains the marker somewhere inside it", () => {
    expect(() => assertSynthetic({ prompt: `Describe ${SYNTHETIC_MARKER}-0123456789abcdef` })).toThrow();
  });

  it("names every offending field at once", () => {
    try {
      assertSynthetic({ a: "real text", b: `${SYNTHETIC_MARKER}-0123456789abcdef`, c: "more text" });
      expect.unreachable("expected a non-synthetic fixture error");
    } catch (error) {
      expect((error as Error).message).toContain("a");
      expect((error as Error).message).toContain("c");
    }
  });
});
