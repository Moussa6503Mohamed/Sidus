import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { ThemeToggle } from "./ThemeToggle";

const STORAGE_KEY = "sidus-theme";

beforeEach(() => {
  window.localStorage.removeItem(STORAGE_KEY);
  document.documentElement.removeAttribute("data-theme");
});

afterEach(() => {
  window.localStorage.removeItem(STORAGE_KEY);
  document.documentElement.removeAttribute("data-theme");
});

describe("ThemeToggle", () => {
  it("renders System by default with no stored preference", async () => {
    render(<ThemeToggle />);
    expect(await screen.findByRole("button", { name: /theme: system/i })).toBeInTheDocument();
  });

  it("reads a previously persisted preference on mount", async () => {
    window.localStorage.setItem(STORAGE_KEY, "dark");
    render(<ThemeToggle />);
    expect(await screen.findByRole("button", { name: /theme: dark/i })).toBeInTheDocument();
  });

  it("cycles system -> light -> dark -> system, persisting and applying data-theme each step", async () => {
    const user = userEvent.setup();
    render(<ThemeToggle />);
    const button = await screen.findByRole("button", { name: /theme: system/i });

    await user.click(button);
    expect(button).toHaveAccessibleName(/theme: light/i);
    expect(document.documentElement.getAttribute("data-theme")).toBe("light");
    expect(window.localStorage.getItem(STORAGE_KEY)).toBe("light");

    await user.click(button);
    expect(button).toHaveAccessibleName(/theme: dark/i);
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
    expect(window.localStorage.getItem(STORAGE_KEY)).toBe("dark");

    await user.click(button);
    expect(button).toHaveAccessibleName(/theme: system/i);
    expect(document.documentElement.hasAttribute("data-theme")).toBe(false);
    expect(window.localStorage.getItem(STORAGE_KEY)).toBeNull();
  });
});
