import { serverAdminCookieHeader } from "@/lib/api/admin.server";
import { adminListPhotographers } from "@/lib/api/admin";
import { PhotographersScreen } from "@/components/admin/photographers-screen";

export const instant = false;


export default async function AdminPhotographersPage({ searchParams }: { searchParams: Promise<{ q?: string; status?: string; offset?: string }> }) {
  const sp = await searchParams;
  const cookie = await serverAdminCookieHeader();
  const q = sp.q || "";
  const status = sp.status || "";
  const offset = Math.max(0, Number(sp.offset || "0") || 0);
  const res = await adminListPhotographers({ q, status, limit: 20, offset }, { server: true, cookie });
  return <PhotographersScreen initial={res.ok ? res.data : null} error={res.ok ? null : res} initialQuery={q} initialStatus={status} />;
}
