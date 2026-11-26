import type { Metadata } from "next";
import { NewShootForm } from "./new-shoot-form";
import { listClients } from "@/lib/api/clients";
import { serverCookieHeader } from "@/lib/api/server";

export const instant = false;
export const metadata: Metadata = { title: "New shoot | Photobooth", robots: { index: false, follow: false } };

export default async function NewShootPage() {
  const cookie = await serverCookieHeader();
  const clients = await listClients({ limit: 100 }, { server: true, cookie });
  return <NewShootForm initialClients={clients.ok ? clients.data.items : []} />;
}
