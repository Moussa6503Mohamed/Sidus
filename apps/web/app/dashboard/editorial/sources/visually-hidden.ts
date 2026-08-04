import type { CSSProperties } from "react";

/** Standard visually-hidden-but-screen-reader-accessible style, used where a caption or label
 * must exist for assistive tech without duplicating a visible heading. */
export const visuallyHidden: CSSProperties = {
  position: "absolute",
  width: "1px",
  height: "1px",
  padding: 0,
  margin: "-1px",
  overflow: "hidden",
  clip: "rect(0, 0, 0, 0)",
  whiteSpace: "nowrap",
  border: 0,
};
