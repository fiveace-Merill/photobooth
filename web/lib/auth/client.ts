import type { AuthError, Photographer } from "./types";

async function parseJson<T>(res: Response): Promise<T> {
  return (await res.json()) as T;
}

async function readError(res: Response): Promise<AuthError> {
  return parseJson<AuthError>(res).catch(() => ({ error: "Request failed" }));
}

export async function signup(input: {
  email: string;
  password: string;
  business_name: string;
  slug: string;
}): Promise<{ ok: true; photographer: Photographer } | { ok: false; error: string; status: number }> {
  const res = await fetch("/api/v1/auth/signup", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
    credentials: "same-origin",
  });
  if (res.ok) {
    return { ok: true, photographer: await parseJson<Photographer>(res) };
  }
  const body = await readError(res);
  return { ok: false, error: body.error || "Signup failed", status: res.status };
}

export async function login(input: {
  email: string;
  password: string;
}): Promise<{ ok: true; photographer: Photographer } | { ok: false; error: string; status: number }> {
  const res = await fetch("/api/v1/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
    credentials: "same-origin",
  });
  if (res.ok) {
    return { ok: true, photographer: await parseJson<Photographer>(res) };
  }
  const body = await readError(res);
  return { ok: false, error: body.error || "Login failed", status: res.status };
}

export async function logout(): Promise<void> {
  await fetch("/api/v1/auth/logout", {
    method: "POST",
    credentials: "same-origin",
  });
}
