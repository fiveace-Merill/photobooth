import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { ClientDetail } from "./client-detail";
import { getClient } from "@/lib/api/clients";
import { listShoots } from "@/lib/api/shoots";
import { serverCookieHeader } from "@/lib/api/server";

export const instant = false;
export const metadata: Metadata = { title: "Client | Photobooth", robots: { index: false, follow: false } };

type Props = { params: Promise<{ id: string }> };

export default async function ClientDetailPage({ params }: Props) {
  const { id } = await params;
  const cookie = await serverCookieHeader();
  const [clientRes, shootsRes] = await Promise.all([
    getClient(id, { server: true, cookie }),
    listShoots({ clientId: id, limit: 100, sort: "created" }, { server: true, cookie }),
  ]);
  if (!clientRes.ok) {
    if (clientRes.status === 404) notFound();
    return <div className="text-sm text-destructive">Could not load client.</div>;
  }
  return <ClientDetail client={clientRes.data} shoots={shootsRes.ok ? shootsRes.data.items : []} />;
}
