import type { Metadata } from "next";
import { redirect } from "next/navigation";
import "../globals.css";
import { bricolageGrotesque } from "@/lib/fonts";
import { getCurrentPhotographer } from "@/lib/auth/server";
import { DashboardShell } from "@/components/dashboard/dashboard-shell";
import { Toaster } from "@/components/ui/sonner";

export const metadata: Metadata = {
  title: "Dashboard | Photobooth",
  robots: { index: false, follow: false },
};

export default async function DashboardRootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  const photographer = await getCurrentPhotographer();
  if (!photographer) redirect("/login");

  return (
    <html lang="en" className={`${bricolageGrotesque.variable} h-full antialiased`}>
      <body className="min-h-full bg-background text-foreground">
        <DashboardShell photographer={photographer}>{children}</DashboardShell>
        <Toaster />
      </body>
    </html>
  );
}
