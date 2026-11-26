import { cookies } from "next/headers";
import type { Photographer } from "./types";

const apiOrigin = process.env.API_URL ?? process.env.API_ORIGIN ?? "http://localhost:8080";

export async function getCurrentPhotographer(): Promise<Photographer | null> {
  const cookieStore = await cookies();
  const cookieHeader = cookieStore
    .getAll()
    .map((c) => `${c.name}=${c.value}`)
    .join("; ");

  const res = await fetch(`${apiOrigin}/api/v1/auth/me`, {
    headers: cookieHeader ? { cookie: cookieHeader } : {},
    cache: "no-store",
  });

  if (!res.ok) return null;
  return (await res.json()) as Photographer;
}
