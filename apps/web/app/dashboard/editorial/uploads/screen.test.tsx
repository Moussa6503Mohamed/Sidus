import { render, screen, waitFor } from "@testing-library/react";
import { vi, it, expect, afterEach } from "vitest";
import { PrivateUploadsScreen } from "./screen";

afterEach(()=>vi.restoreAllMocks());
it("unknown role makes zero intake requests",async()=>{
 const fetchSpy=vi.spyOn(globalThis,"fetch"); render(<PrivateUploadsScreen role="unknown"/>);
 expect(screen.getByText("Admin access required.")).toBeInTheDocument();
 await waitFor(()=>expect(fetchSpy).not.toHaveBeenCalled());
});
