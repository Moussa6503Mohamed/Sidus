import { REQUIRED_APPROVAL_FIELDS, type ContentSource, type RequiredApprovalField } from "./types";

/** Client-side mirror of Core's `MissingApprovalFields` (services/core/internal/contentsource),
 * used only to warn a reviewer before they attempt an approval Core would reject with `422`. */
export function missingApprovalFields(source: ContentSource): RequiredApprovalField[] {
  return REQUIRED_APPROVAL_FIELDS.filter((field) => {
    const value = source[field];
    return typeof value !== "string" || value.trim() === "";
  });
}
