import type { Metadata } from "next";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { StatusChip } from "@/components/dashboard/status-chip";
import { OverdueLabel } from "@/components/dashboard/overdue-label";
import { getShootSummary, listShoots } from "@/lib/api/shoots";
import { serverCookieHeader } from "@/lib/api/server";
import { formatMoney } from "@/lib/money";
import { formatDisplayDate } from "@/lib/dates";

export const instant = false;
export const metadata: Metadata = {
  title: "Overview | Photobooth",
  robots: { index: false, follow: false },
};

export default async function DashboardPage() {
  const cookie = await serverCookieHeader();
  const mode = { server: true, cookie } as const;
  const [summaryRes, overdueRes] = await Promise.all([
    getShootSummary(mode),
    listShoots({ overdue: true, sort: "due", limit: 5 }, mode),
  ]);

  const summary = summaryRes.ok ? summaryRes.data : null;
  const overdue = overdueRes.ok ? overdueRes.data.items : [];
  const hasAnything = summary ? Object.values(summary.counts).some((n) => n > 0) : false;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Overview</h1>
        <p className="mt-1 text-sm text-muted-foreground">Track shoot progress and what needs attention.</p>
      </div>

      {!summaryRes.ok || !overdueRes.ok ? (
        <Card><CardContent className="p-5"><p className="text-sm text-destructive">Could not load overview. Please refresh to try again.</p></CardContent></Card>
      ) : null}

      {summary ? (
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
          {(["shot", "editing", "ready", "delivered", "expired"] as const).map((status) => (
            <Card key={status}>
              <CardHeader className="pb-2"><CardTitle className="text-sm font-medium text-muted-foreground flex items-center justify-between">{status[0].toUpperCase() + status.slice(1)} <StatusChip status={status} /></CardTitle></CardHeader>
              <CardContent><p className="text-2xl font-semibold">{summary.counts[status]}</p></CardContent>
            </Card>
          ))}
        </div>
      ) : null}

      <section className="space-y-3">
        <div className="flex items-center justify-between gap-3">
          <h2 className="text-lg font-semibold tracking-tight">Overdue</h2>
          <Button render={<Link href="/dashboard/shoots?overdue=true" />} variant="outline" className="min-h-11">View all shoots</Button>
        </div>

        {!hasAnything && overdue.length === 0 ? (
          <Card><CardContent className="p-6"><p className="text-sm text-muted-foreground">No shoots yet. When you add shoots, overdue work will appear here.</p></CardContent></Card>
        ) : overdue.length === 0 ? (
          <Card><CardContent className="p-6"><p className="text-sm text-muted-foreground">Nothing overdue. You’re all caught up.</p></CardContent></Card>
        ) : (
          <div className="space-y-3">
            {overdue.map((shoot) => (
              <Card key={shoot.id}>
                <CardContent className="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between">
                  <div className="min-w-0">
                    <Link href={`/dashboard/shoots/${shoot.id}`} className="font-medium hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-sm">
                      {shoot.title}
                    </Link>
                    <p className="text-sm text-muted-foreground">{shoot.clientName ?? "Client"}{shoot.dueDate ? ` · Due ${formatDisplayDate(shoot.dueDate)}` : ""} · {formatMoney(shoot.priceMinor, shoot.currency)}</p>
                  </div>
                  <div className="flex items-center gap-3">
                    <StatusChip status={shoot.status} />
                    <OverdueLabel daysOverdue={shoot.daysOverdue} />
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
