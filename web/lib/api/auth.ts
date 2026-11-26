import type { Photographer } from "@/lib/auth/types";

export type AuthApiError = {
  ok: false;
  status: number;
  code?: string;
  error: string;
};

export type AuthApiOk<T> = { ok: true; data: T };
export type AuthApiResult<T> = AuthApiOk<T> | AuthApiError;

export interface AuthApi {
  signup(input: {
    email: string;
    password: string;
    business_name: string;
    slug: string;
  }): Promise<AuthApiResult<Photographer>>;
  login(input: { email: string; password: string }): Promise<AuthApiResult<Photographer>>;
  logout(): Promise<void>;
  me(): Promise<AuthApiResult<Photographer>>;
}

async function parseJson<T>(res: Response): Promise<T> {
  return (await res.json()) as T;
}

async function toError(res: Response): Promise<AuthApiError> {
  const body = await parseJson<{ error?: string; code?: string }>(res).catch(
    () => ({}) as { error?: string; code?: string },
  );
  return {
    ok: false,
    status: res.status,
    code: body.code,
    error: body.error || "Request failed",
  };
}

export const authApi: AuthApi = {
  async signup(input) {
    const res = await fetch("/api/v1/auth/signup", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
      credentials: "same-origin",
    });
    if (res.ok) return { ok: true, data: await parseJson<Photographer>(res) };
    return toError(res);
  },

  async login(input) {
    const res = await fetch("/api/v1/auth/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
      credentials: "same-origin",
    });
    if (res.ok) return { ok: true, data: await parseJson<Photographer>(res) };
    return toError(res);
  },

  async logout() {
    await fetch("/api/v1/auth/logout", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "same-origin",
    }).catch(() => undefined);
  },

  async me() {
    const res = await fetch("/api/v1/auth/me", { credentials: "same-origin" });
    if (res.ok) return { ok: true, data: await parseJson<Photographer>(res) };
    return toError(res);
  },
};
