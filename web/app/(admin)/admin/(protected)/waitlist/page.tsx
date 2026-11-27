import { serverAdminCookieHeader } from "@/lib/api/admin.server";
import { adminListWaitlist } from "@/lib/api/admin";
import { WaitlistScreen } from "@/components/admin/waitlist-screen";

export const instant = false;


export default async function AdminWaitlistPage({
  searchParams,
}: {
  searchParams: Promise<{ status?: string; q?: string; offset?: string }>;
}) {
  const sp = await searchParams;
  const cookie = await serverAdminCookieHeader();
  const status = sp.status || "";
  const q = sp.q || "";
  const offset = Math.max(0, Number(sp.offset || "0") || 0);
  const res = await adminListWaitlist({ status, q, limit: 20, offset }, { server: true, cookie });
  return <WaitlistScreen initial={res.ok ? res.data : null} error={res.ok ? null : res} initialStatus={status} initialQuery={q} />;
}
