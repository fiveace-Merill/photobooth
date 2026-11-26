import type { Metadata } from "next";
import { ShootsScreen } from "./shoots-screen";
import { listShoots } from "@/lib/api/shoots";
import { serverCookieHeader } from "@/lib/api/server";

export const instant = false;
export const metadata: Metadata = { title: "Shoots | Photobooth", robots: { index: false, follow: false } };

type Props = { searchParams: Promise<Record<string, string | string[] | undefined>> };

export default async function ShootsPage({ searchParams }: Props) {
  const sp = await searchParams;
  const q = typeof sp.q === "string" ? sp.q : "";
  const status = typeof sp.status === "string" ? sp.status : "";
  const overdue = sp.overdue === "true";
  const offset = Math.max(0, Number(typeof sp.offset === "string" ? sp.offset : 0) || 0);
  const cookie = await serverCookieHeader();
  const res = await listShoots({ q, status, overdue: overdue ? true : undefined, limit: 25, offset, sort: "due" }, { server: true, cookie });
  return <ShootsScreen initial={res.ok ? res.data : null} error={res.ok ? null : res} initialQuery={{ q, status, overdue, offset }} />;
}
