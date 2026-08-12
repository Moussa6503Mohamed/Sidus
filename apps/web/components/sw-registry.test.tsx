import { render } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { SwRegistry } from "./sw-registry";

describe("SwRegistry", () => {
  it("registers service worker on load", () => {
    const register = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(global.navigator, "serviceWorker", {
      value: { register },
      writable: true,
    });
    
    render(<SwRegistry />);
    window.dispatchEvent(new Event("load"));
    
    expect(register).toHaveBeenCalledWith("/sw.js");
  });
});
