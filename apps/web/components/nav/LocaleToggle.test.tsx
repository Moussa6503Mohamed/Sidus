import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { LocaleToggle } from "./LocaleToggle";
import { useRouter } from "next/navigation";

vi.mock("next/navigation", () => ({
  useRouter: vi.fn(() => ({ refresh: vi.fn() })),
}));

describe("LocaleToggle", () => {
  it("toggles locale cookie and refreshes router", async () => {
    const refresh = vi.fn();
    vi.mocked(useRouter).mockReturnValue({ refresh } as any);
    const user = userEvent.setup();
    render(<LocaleToggle currentLocale="en" />);
    
    await user.click(screen.getByRole("button", { name: "Toggle language" }));
    
    expect(document.cookie).toContain("sidus_locale=ar");
    expect(refresh).toHaveBeenCalled();
  });
});
