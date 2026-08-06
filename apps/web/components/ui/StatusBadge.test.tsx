import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { STATUS_VISUALS, type LifecycleStatus } from "@/lib/design/status";
import { StatusBadge } from "./StatusBadge";

describe("StatusBadge", () => {
  it.each(Object.keys(STATUS_VISUALS) as LifecycleStatus[])(
    "renders the %s status with both an icon and a visible text label",
    (status) => {
      const { container } = render(<StatusBadge status={status} />);
      const visual = STATUS_VISUALS[status];

      expect(screen.getByText(visual.label)).toBeInTheDocument();
      // Icon shape is a real, non-decorative-only signal alongside colour: an aria-hidden svg is
      // present so colour is never the sole way a status is distinguished.
      expect(container.querySelector("svg[aria-hidden='true']")).toBeInTheDocument();
    },
  );

  it("marks retired's label with a strikethrough treatment, not colour alone", () => {
    render(<StatusBadge status="retired" />);
    const label = screen.getByText("Retired");
    expect(label.className).toContain("struck");
  });

  it("uses a dashed border for draft — a shape/border signal independent of colour", () => {
    const { container } = render(<StatusBadge status="draft" />);
    const badge = container.querySelector("[data-border='dashed']");
    expect(badge).not.toBeNull();
  });
});
