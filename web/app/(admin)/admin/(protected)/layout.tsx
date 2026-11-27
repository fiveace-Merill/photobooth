import { redirect } from "next/navigation";
import { getCurrentAdmin } from "@/lib/api/admin.server";
import { AdminShell } from "@/components/admin/admin-shell";

export const instant = false;

export default async function AdminProtectedLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  const admin = await getCurrentAdmin();
  if (!admin) redirect("/admin/login");
  return <AdminShell admin={admin}>{children}</AdminShell>;
}
