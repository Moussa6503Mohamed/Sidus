import { describe, expect, it } from "vitest";
import { canReview, isEditorialRole, parseSidusRole } from "./permissions";

describe("parseSidusRole", () => {
  it("parses each known role", () => {
    expect(parseSidusRole("learner")).toBe("learner");
    expect(parseSidusRole("editor")).toBe("editor");
    expect(parseSidusRole("reviewer")).toBe("reviewer");
    expect(parseSidusRole("admin")).toBe("admin");
  });

  it("trims whitespace", () => {
    expect(parseSidusRole("  admin  ")).toBe("admin");
  });

  it("denies by default on missing, non-string, or unrecognized values", () => {
    expect(parseSidusRole(undefined)).toBe("unknown");
    expect(parseSidusRole(null)).toBe("unknown");
    expect(parseSidusRole(42)).toBe("unknown");
    expect(parseSidusRole("")).toBe("unknown");
    expect(parseSidusRole("superadmin")).toBe("unknown");
  });
});

describe("isEditorialRole", () => {
  it("is true only for editor, reviewer, admin", () => {
    expect(isEditorialRole("editor")).toBe(true);
    expect(isEditorialRole("reviewer")).toBe(true);
    expect(isEditorialRole("admin")).toBe(true);
    expect(isEditorialRole("learner")).toBe(false);
    expect(isEditorialRole("unknown")).toBe(false);
  });
});

describe("canReview", () => {
  it("is true only for reviewer, admin", () => {
    expect(canReview("reviewer")).toBe(true);
    expect(canReview("admin")).toBe(true);
    expect(canReview("editor")).toBe(false);
    expect(canReview("learner")).toBe(false);
    expect(canReview("unknown")).toBe(false);
  });
});
