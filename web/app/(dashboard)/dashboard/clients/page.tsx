import type { Metadata } from "next";
import { ClientsScreen } from "./clients-screen";
import { listClients } from "@/lib/api/clients";
import { serverCookieHeader } from "@/lib/api/server";

export const instant = false;
export const metadata: Metadata = { title: "Clients | Photobooth", robots: { index: false, follow: false } };

type Props = { searchParams: Promise<Record<string, string | string[] | undefined>> };

export default async function ClientsPage({ searchParams }: Props) {
  const sp = await searchParams;
  const q = typeof sp.q === "string" ? sp.q : "";
  const offset = Math.max(0, Number(typeof sp.offset === "string" ? sp.offset : 0) || 0);
  const cookie = await serverCookieHeader();
  const res = await listClients({ q, limit: 25, offset }, { server: true, cookie });
  return <ClientsScreen initial={res.ok ? res.data : null} error={res.ok ? null : res} initialQuery={{ q, offset }} />;
}
