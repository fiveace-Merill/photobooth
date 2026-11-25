import type { Metadata } from "next";
import "../globals.css";
import { bricolageGrotesque } from "@/lib/fonts";

export const metadata: Metadata = {
  title: "Dashboard | Photobooth",
};

export default function DashboardRootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" className={`${bricolageGrotesque.variable} h-full antialiased`}>
      <body className="min-h-full bg-background text-foreground">{children}</body>
    </html>
  );
}
