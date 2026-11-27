import { authApi, type AuthApiError, type AuthConfig, type InvitationInfo } from "@/lib/api/auth";
import type { Photographer } from "./types";

type Result = { ok: true; photographer: Photographer } | { ok: false; error: string; code?: string; status: number };
type ConfigResult = { ok: true; config: AuthConfig } | { ok: false; error: string; code?: string; status: number };
type InvitationResult = { ok: true; invitation: InvitationInfo } | { ok: false; error: string; code?: string; status: number };

function toResult(res: { ok: true; data: Photographer } | AuthApiError): Result {
  if (res.ok) return { ok: true, photographer: res.data };
  return { ok: false, error: res.error, code: res.code, status: res.status };
}

export async function signup(input: {
  email: string;
  password: string;
  business_name: string;
  slug: string;
  inviteToken?: string;
}): Promise<Result> {
  return toResult(await authApi.signup(input));
}

export async function login(input: { email: string; password: string }): Promise<Result> {
  return toResult(await authApi.login(input));
}

export async function logout(): Promise<void> {
  await authApi.logout();
}

export async function me(): Promise<Result> {
  return toResult(await authApi.me());
}

export async function authConfig(): Promise<ConfigResult> {
  const res = await authApi.config();
  if (res.ok) return { ok: true, config: res.data };
  return { ok: false, error: res.error, code: res.code, status: res.status };
}

export async function getInvitation(token: string): Promise<InvitationResult> {
  const res = await authApi.getInvitation(token);
  if (res.ok) return { ok: true, invitation: res.data };
  return { ok: false, error: res.error, code: res.code, status: res.status };
}
