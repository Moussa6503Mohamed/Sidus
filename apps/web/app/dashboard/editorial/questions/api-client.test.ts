import { describe, expect, it } from "vitest";
import { ApiError, apiErrorMessage } from "./api-client";

describe("apiErrorMessage", () => {
  it.each([
    ["unverified_node", /not verified/i],
    ["unapproved_source", /no longer approved/i],
    ["invalid_lifecycle_transition", /lifecycle state/i],
    ["missing_verified_rubric", /at least one rubric/i],
    ["missing_current_verified_rubric", /current revision/i],
    ["invalid_canonical_rubric", /does not belong/i],
    ["canonical_rubric_not_verified", /not verified/i],
    ["canonical_rubric_not_current", /older question revision/i],
    ["canonical_rubric_already_set", /cannot be replaced/i],
    ["invalid_rubric", /criteria are invalid/i],
    ["duplicate_rubric_version", /duplicate/i],
    ["no_changes", /no question fields changed/i],
  ])("explains Core error %s", (code, pattern) => {
    expect(apiErrorMessage(new ApiError(400, code, "fallback"))).toMatch(pattern);
  });
});
