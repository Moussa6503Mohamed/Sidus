import { describe, expect, it } from "vitest";
import { ApiError, apiErrorMessage } from "./api-client";

describe("apiErrorMessage", () => {
  it.each([
    ["unverified_node", /not verified/i],
    ["unapproved_source", /no longer approved/i],
    ["invalid_lifecycle_transition", /lifecycle state/i],
    ["missing_verified_rubric", /at least one rubric/i],
    ["missing_current_verified_rubric", /current revision/i],
    ["invalid_rubric", /criteria are invalid/i],
    ["duplicate_rubric_version", /duplicate/i],
    ["no_changes", /no question fields changed/i],
  ])("explains Core error %s", (code, pattern) => {
    expect(apiErrorMessage(new ApiError(400, code, "fallback"))).toMatch(pattern);
  });
});
