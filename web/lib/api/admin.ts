import { apiRequest } from "./request";
import type { ApiResult } from "./errors";

export type AdminMe = { id: string; email: string; status: string; totpConfirmed: boolean };

export type AdminLoginResponse = { next: "totp" | "totp_setup" | "ok" };
export type AdminTOTPStartResponse = { otpauthUri: string; secret: string };
export type AdminTOTPConfirmResponse = { recoveryCodes: string[] };

export type AdminOverview = {
  photographers: { total: number; active: number; suspended: number };
  shootsTotal: number;
  photosTotal: number;
  shootsDeliveredLast30Days: number;
  storageUsedBytesTotal: number;
  waitlistNew: number;
  pendingInvitations: number;
};

export type WaitlistEntry = {
  id: string;
  name: string;
  contactEmail: string;
  contactPhone: string;
  photographyType: string;
  shootsPerMonth: string;
  consentAt: string;
  consentVersion: string;
  status: "new" | "invited" | "declined" | "spam" | string;
  invitationId: string | null;
  invitedAt: string | null;
  notes: string;
  createdAt: string;
  updatedAt: string;
};

export type WaitlistListResponse = { items: WaitlistEntry[]; total: number; limit: number; offset: number };

export type Invitation = {
  id: string;
  email: string;
  planId: string;
  note: string;
  expiresAt: string;
  createdAt: string;
  usedAt?: string | null;
  revokedAt?: string | null;
  status: "pending" | "used" | "revoked" | "expired" | string;
};

export type InvitationCreateResponse = { url: string; expiresAt: string; emailStatus: string; invitation?: Invitation };

export type AdminPhotographer = {
  id: string;
  email: string;
  businessName: string;
  slug: string;
  planId: string;
  status: "active" | "suspended" | string;
  storageUsedBytes: number;
  storageQuotaBytes: number;
  createdAt: string;
  lastLoginAt?: string | null;
  suspendedAt?: string | null;
  suspendedReason?: string;
  shoots: number;
  photos: number;
  delivered: number;
};

export type AdminPhotographersResponse = { items: AdminPhotographer[]; total: number; limit: number; offset: number };

export type AuditEntry = {
  id: string;
  adminId: string;
  actor: string;
  action: string;
  targetType: string;
  targetId: string;
  meta: Record<string, unknown>;
  ip: string;
  createdAt: string;
};

export type AuditListResponse = { items: AuditEntry[]; total: number; limit: number; offset: number };

type Mode = { server: true; cookie?: string } | { server?: false };
const browser: Mode = { server: false };

export function adminLogin(input: { email: string; password: string }) {
  return apiRequest<AdminLoginResponse>("/api/v1/admin/auth/login", { method: "POST", body: JSON.stringify(input) }, browser);
}
export function adminTOTPVerify(input: { code?: string; recoveryCode?: string }) {
  return apiRequest<{ ok: true }>("/api/v1/admin/auth/totp/verify", { method: "POST", body: JSON.stringify(input) }, browser);
}
export function adminTOTPSetupStart() {
  return apiRequest<AdminTOTPStartResponse>("/api/v1/admin/auth/totp/setup/start", { method: "POST", body: JSON.stringify({}) }, browser);
}
export function adminTOTPSetupConfirm(input: { code: string }) {
  return apiRequest<AdminTOTPConfirmResponse>("/api/v1/admin/auth/totp/setup/confirm", { method: "POST", body: JSON.stringify(input) }, browser);
}
export function adminLogout() {
  return apiRequest<void>("/api/v1/admin/auth/logout", { method: "POST" }, browser);
}
export function adminMe(mode: Mode = browser) {
  return apiRequest<AdminMe>("/api/v1/admin/auth/me", { method: "GET" }, mode);
}
export function adminChangePassword(input: { current: string; new: string }) {
  return apiRequest<void>("/api/v1/admin/auth/password", { method: "POST", body: JSON.stringify(input) }, browser);
}

export function adminOverview(mode: Mode = browser) {
  return apiRequest<AdminOverview>("/api/v1/admin/overview", { method: "GET" }, mode);
}

export function adminListWaitlist(params: { status?: string; q?: string; limit?: number; offset?: number }, mode: Mode = browser) {
  const sp = new URLSearchParams();
  if (params.status) sp.set("status", params.status);
  if (params.q) sp.set("q", params.q);
  if (params.limit != null) sp.set("limit", String(params.limit));
  if (params.offset != null) sp.set("offset", String(params.offset));
  const q = sp.toString();
  return apiRequest<WaitlistListResponse>(`/api/v1/admin/waitlist${q ? `?${q}` : ""}`, { method: "GET" }, mode);
}
export function adminGetWaitlist(id: string) {
  return apiRequest<WaitlistEntry>(`/api/v1/admin/waitlist/${id}`, { method: "GET" }, browser);
}
export function adminPatchWaitlist(id: string, input: { notes?: string; email?: string; phone?: string }) {
  return apiRequest<WaitlistEntry>(`/api/v1/admin/waitlist/${id}`, { method: "PATCH", body: JSON.stringify(input) }, browser);
}
export function adminInviteWaitlist(id: string, input: { email?: string; planId: string; days: number; send: boolean }) {
  return apiRequest<InvitationCreateResponse>(`/api/v1/admin/waitlist/${id}/invite`, { method: "POST", body: JSON.stringify(input) }, browser);
}
export function adminDeclineWaitlist(id: string) {
  return apiRequest<WaitlistEntry>(`/api/v1/admin/waitlist/${id}/decline`, { method: "POST", body: JSON.stringify({}) }, browser);
}
export function adminSpamWaitlist(id: string) {
  return apiRequest<WaitlistEntry>(`/api/v1/admin/waitlist/${id}/spam`, { method: "POST", body: JSON.stringify({}) }, browser);
}
export function adminDeleteWaitlist(id: string) {
  return apiRequest<void>(`/api/v1/admin/waitlist/${id}`, { method: "DELETE" }, browser);
}
export async function adminExportWaitlistCsv(): Promise<ApiResult<Response>> {
  try {
    const res = await fetch("/api/v1/admin/waitlist/export.csv", { credentials: "same-origin", cache: "no-store" });
    if (res.ok) return { ok: true, data: res };
    const body = await res.json().catch(() => ({}));
    return { ok: false, status: res.status, code: body.code, error: body.error || "Request failed" };
  } catch {
    return { ok: false, status: 0, error: "Network error" };
  }
}

export function adminListInvitations(status = "") {
  const sp = new URLSearchParams();
  if (status) sp.set("status", status);
  const q = sp.toString();
  return apiRequest<{ items: Invitation[] }>(`/api/v1/admin/invitations${q ? `?${q}` : ""}`, { method: "GET" }, browser);
}
export function adminCreateInvitation(input: { email: string; planId: string; days: number; note?: string; send: boolean }) {
  return apiRequest<InvitationCreateResponse>("/api/v1/admin/invitations", { method: "POST", body: JSON.stringify(input) }, browser);
}
export function adminRevokeInvitation(id: string) {
  return apiRequest<Invitation>(`/api/v1/admin/invitations/${id}/revoke`, { method: "POST", body: JSON.stringify({}) }, browser);
}
export function adminResendInvitation(id: string, input: { days: number; send: boolean }) {
  return apiRequest<InvitationCreateResponse>(`/api/v1/admin/invitations/${id}/resend`, { method: "POST", body: JSON.stringify(input) }, browser);
}

export function adminListPhotographers(params: { q?: string; status?: string; limit?: number; offset?: number }, mode: Mode = browser) {
  const sp = new URLSearchParams();
  if (params.q) sp.set("q", params.q);
  if (params.status) sp.set("status", params.status);
  if (params.limit != null) sp.set("limit", String(params.limit));
  if (params.offset != null) sp.set("offset", String(params.offset));
  const q = sp.toString();
  return apiRequest<AdminPhotographersResponse>(`/api/v1/admin/photographers${q ? `?${q}` : ""}`, { method: "GET" }, mode);
}
export function adminGetPhotographer(id: string, mode: Mode = browser) {
  return apiRequest<AdminPhotographer>(`/api/v1/admin/photographers/${id}`, { method: "GET" }, mode);
}
export function adminSuspendPhotographer(id: string, reason: string) {
  return apiRequest<{ ok: boolean }>(`/api/v1/admin/photographers/${id}/suspend`, { method: "POST", body: JSON.stringify({ reason }) }, browser);
}
export function adminUnsuspendPhotographer(id: string) {
  return apiRequest<{ ok: boolean }>(`/api/v1/admin/photographers/${id}/unsuspend`, { method: "POST", body: JSON.stringify({}) }, browser);
}
export function adminChangePhotographerPlan(id: string, planId: string) {
  return apiRequest<{ ok: boolean }>(`/api/v1/admin/photographers/${id}/plan`, { method: "PATCH", body: JSON.stringify({ planId }) }, browser);
}

export function adminListAudit(params: { action?: string; limit?: number; offset?: number }, mode: Mode = browser) {
  const sp = new URLSearchParams();
  if (params.action) sp.set("action", params.action);
  if (params.limit != null) sp.set("limit", String(params.limit));
  if (params.offset != null) sp.set("offset", String(params.offset));
  const q = sp.toString();
  return apiRequest<AuditListResponse>(`/api/v1/admin/audit${q ? `?${q}` : ""}`, { method: "GET" }, mode);
}
