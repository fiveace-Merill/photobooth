import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { ShootDetail } from "./shoot-detail";
import { getShoot } from "@/lib/api/shoots";
import { getClient } from "@/lib/api/clients";
import { serverCookieHeader } from "@/lib/api/server";

export const instant = false;
export const metadata: Metadata = { title: "Shoot | Photobooth", robots: { index: false, follow: false } };

type Props = { params: Promise<{ id: string }> };

export default async function ShootDetailPage({ params }: Props) {
  const { id } = await params;
  const cookie = await serverCookieHeader();
  const shootRes = await getShoot(id, { server: true, cookie });
  if (!shootRes.ok) {
    if (shootRes.status === 404) notFound();
    return <div className="text-sm text-destructive">Could not load shoot.</div>;
  }
  const clientRes = await getClient(shootRes.data.clientId, { server: true, cookie });
  return <ShootDetail shoot={shootRes.data} client={clientRes.ok ? clientRes.data : null} />;
}
