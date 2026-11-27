import { notFound } from "next/navigation";
import { serverAdminCookieHeader } from "@/lib/api/admin.server";
import { adminGetPhotographer } from "@/lib/api/admin";
import { PhotographerDetail } from "@/components/admin/photographer-detail";

export const instant = false;


export default async function AdminPhotographerDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const cookie = await serverAdminCookieHeader();
  const res = await adminGetPhotographer(id, { server: true, cookie });
  if (!res.ok) {
    if (res.status === 404) notFound();
    return <div className="text-sm text-destructive">Could not load photographer.</div>;
  }
  return <PhotographerDetail initial={res.data} />;
}
