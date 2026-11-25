import type { Metadata } from "next";
import "../globals.css";
import { bricolageGrotesque } from "@/lib/fonts";

export const metadata: Metadata = {
  title: "Photobooth",
  description: "Photo delivery for photographers and their clients.",
};

export default function SiteRootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" className={`${bricolageGrotesque.variable} h-full antialiased`}>
      <body className="min-h-full flex flex-col">{children}</body>
    </html>
  );
}
