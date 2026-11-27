import { serverAdminCookieHeader } from "@/lib/api/admin.server";
import { InvitationsScreen } from "@/components/admin/invitations-screen";

export const instant = false;


export default async function AdminInvitationsPage() {
  const cookie = await serverAdminCookieHeader();
  // no server mode helper for listInvitations; fetch directly
  const res = await fetch(`${process.env.API_URL ?? process.env.API_ORIGIN ?? "http://localhost:8080"}/api/v1/admin/invitations`, {
    headers: cookie ? { cookie } : {},
    cache: "no-store",
  });
  const data = res.ok ? await res.json() : null;
  return <InvitationsScreen initial={data} error={res.ok ? null : { ok: false, status: res.status, code: undefined, error: "Request failed" }} />;
}
