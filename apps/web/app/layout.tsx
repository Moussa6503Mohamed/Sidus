import type { Metadata } from "next";

export const metadata: Metadata = { title: "Sidus Observatory", description: "Sidus academic preparation" };

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="en"><body>{children}</body></html>;
}
