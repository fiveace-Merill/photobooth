import { toApiError, type ApiResult } from "./errors";

type FetchMode = { server: true; cookie?: string } | { server?: false };

function apiBase(mode: FetchMode): string {
  if (mode.server) {
    return process.env.API_URL ?? process.env.API_ORIGIN ?? "http://localhost:8080";
  }
  return "";
}

export async function apiRequest<T>(
  path: string,
  init: RequestInit,
  mode: FetchMode,
): Promise<ApiResult<T>> {
  const base = apiBase(mode);
  const headers = new Headers(init.headers);
  if (mode.server && mode.cookie) headers.set("cookie", mode.cookie);
  if (init.body && !headers.has("Content-Type")) headers.set("Content-Type", "application/json");

  const res = await fetch(`${base}${path}`, {
    ...init,
    headers,
    cache: "no-store",
    credentials: mode.server ? undefined : "same-origin",
  });

  if (res.status === 204) return { ok: true, data: undefined as T };
  if (res.ok) return { ok: true, data: (await res.json()) as T };
  return toApiError(res);
}
