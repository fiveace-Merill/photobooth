import { cookies } from "next/headers";
import type { AdminMe } from "./admin";

const apiOrigin = process.env.API_URL ?? process.env.API_ORIGIN ?? "http://localhost:8080";

export async function serverAdminCookieHeader(): Promise<string> {
  const store = await cookies();
  return store.getAll().map((c) => `${c.name}=${c.value}`).join("; ");
}

export async function getCurrentAdmin(): Promise<AdminMe | null> {
  const cookie = await serverAdminCookieHeader();
  try {
    const res = await fetch(`${apiOrigin}/api/v1/admin/auth/me`, {
      headers: cookie ? { cookie } : {},
      cache: "no-store",
    });
    if (!res.ok) return null;
    return (await res.json()) as AdminMe;
  } catch {
    return null;
  }
}
