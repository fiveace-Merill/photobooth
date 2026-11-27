import { serverAdminCookieHeader } from "@/lib/api/admin.server";
import { adminListAudit } from "@/lib/api/admin";
import { AuditScreen } from "@/components/admin/audit-screen";

export const instant = false;


export default async function AdminAuditPage({ searchParams }: { searchParams: Promise<{ action?: string; offset?: string }> }) {
  const sp = await searchParams;
  const cookie = await serverAdminCookieHeader();
  const action = sp.action || "";
  const offset = Math.max(0, Number(sp.offset || "0") || 0);
  const res = await adminListAudit({ action, limit: 25, offset }, { server: true, cookie });
  return <AuditScreen initial={res.ok ? res.data : null} error={res.ok ? null : res} initialAction={action} />;
}
