import { render, screen, waitFor } from "@testing-library/react";
import { vi, it, expect, afterEach } from "vitest";
import { PrivateUploadsScreen } from "./screen";

afterEach(()=>vi.restoreAllMocks());
it("unknown role makes zero intake requests",async()=>{
 const fetchSpy=vi.spyOn(globalThis,"fetch"); render(<PrivateUploadsScreen role="unknown"/>);
 expect(screen.getByText("Admin access required.")).toBeInTheDocument();
 await waitFor(()=>expect(fetchSpy).not.toHaveBeenCalled());
});

it("admin sees branded quarantine controls and an explicit empty state", async () => {
 const fetchSpy=vi.spyOn(globalThis,"fetch").mockResolvedValue({ ok:true, json:async()=>({items:[]}) } as Response);
 render(<PrivateUploadsScreen role="admin"/>);
 expect(screen.getByRole("heading",{name:"Private PDF intake"})).toBeInTheDocument();
 expect(screen.getByRole("button",{name:"Upload to quarantine"})).toBeInTheDocument();
 expect(await screen.findByText("No private uploads yet.")).toBeInTheDocument();
 expect(fetchSpy).toHaveBeenCalledTimes(2);
});
