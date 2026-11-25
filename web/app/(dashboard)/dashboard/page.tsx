import { redirect } from "next/navigation";
import { getCurrentPhotographer } from "@/lib/auth/server";
import { LogoutButton } from "./logout-button";

// Auth check reads cookies/fetches current session; opt out of instant/static shell validation.
export const instant = false;

export default async function DashboardPage() {
  const photographer = await getCurrentPhotographer();
  if (!photographer) {
    redirect("/login");
  }

  return (
    <main className="mx-auto max-w-4xl px-6 py-16">
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="text-sm font-medium text-muted-foreground">Dashboard</p>
          <h1 className="mt-3 text-4xl font-semibold tracking-tight">
            {photographer.business_name}
          </h1>
          <p className="mt-3 text-muted-foreground">
            Signed in as {photographer.email}
          </p>
        </div>
        <LogoutButton />
      </div>

      <section className="mt-10 rounded-xl border border-border bg-card p-6">
        <h2 className="text-lg font-semibold">Next steps</h2>
        <p className="mt-2 text-sm text-muted-foreground">
          Your account is ready. Client, shoot, and delivery management will appear here as the next slices are built.
        </p>
      </section>
    </main>
  );
}
