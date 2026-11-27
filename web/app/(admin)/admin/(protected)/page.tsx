import { adminOverview } from "@/lib/api/admin";
import { serverAdminCookieHeader } from "@/lib/api/admin.server";
import { messageForAdminError } from "@/lib/admin/errors";
import { Card } from "@/components/ui/card";

export const instant = false;


export default async function AdminOverviewPage() {
  const cookie = await serverAdminCookieHeader();
  const res = await adminOverview({ server: true, cookie });
  if (!res.ok) {
    return <div className="text-sm text-destructive">{messageForAdminError(res)}</div>;
  }
  const d = res.data;
  const cards = [
    { label: "New waitlist entries", value: d.waitlistNew },
    { label: "Pending invitations", value: d.pendingInvitations },
    { label: "Active photographers", value: d.photographers.active },
    { label: "Suspended photographers", value: d.photographers.suspended },
    { label: "Shoots delivered (30 days)", value: d.shootsDeliveredLast30Days },
    { label: "Storage used", value: formatBytes(d.storageUsedBytesTotal) },
  ];
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Overview</h1>
        <p className="mt-1 text-sm text-muted-foreground">Platform metadata and aggregate counts only.</p>
      </div>
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        {cards.map((c) => (
          <Card key={c.label} className="p-5">
            <p className="text-sm text-muted-foreground">{c.label}</p>
            <p className="mt-2 text-2xl font-semibold tracking-tight">{c.value}</p>
          </Card>
        ))}
      </div>
      {d.photographers.total === 0 && d.waitlistNew === 0 ? (
        <p className="text-sm text-muted-foreground">No activity yet. New waitlist entries and photographer stats will appear here.</p>
      ) : null}
    </div>
  );
}

function formatBytes(bytes: number): string {
  if (!bytes) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  let v = bytes;
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++; }
  return `${v >= 10 || i === 0 ? v.toFixed(0) : v.toFixed(1)} ${units[i]}`;
}
