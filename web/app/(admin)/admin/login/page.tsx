import { redirect } from "next/navigation";
import { getCurrentAdmin } from "@/lib/api/admin.server";
import { AdminLoginForm } from "@/components/admin/login-form";

export const instant = false;

export default async function AdminLoginPage() {
  const admin = await getCurrentAdmin();
  if (admin) redirect("/admin");

  return (
    <main className="flex min-h-screen items-center justify-center px-4 py-10">
      <div className="w-full max-w-md rounded-2xl border border-border bg-card p-6 shadow-sm sm:p-8">
        <p className="text-sm font-medium text-muted-foreground">Photobooth</p>
        <h1 className="mt-2 text-2xl font-semibold tracking-tight">Admin sign in</h1>
        <p className="mt-2 text-sm text-muted-foreground">Use your admin account. This area is not public.</p>
        <div className="mt-6">
          <AdminLoginForm />
        </div>
      </div>
    </main>
  );
}
