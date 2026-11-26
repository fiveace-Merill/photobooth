import { apiRequest } from "./request";
import type { ApiResult } from "./errors";

export type ShootStatus = "shot" | "editing" | "ready" | "delivered" | "expired";

export type Shoot = {
  id: string;
  clientId: string;
  clientName?: string;
  title: string;
  shootDate?: string;
  status: ShootStatus;
  dueDate?: string;
  priceMinor: number;
  currency: string;
  overdue: boolean;
  daysOverdue: number;
  createdAt: string;
  updatedAt: string;
};

export type ShootListResponse = {
  items: Shoot[];
  total: number;
  limit: number;
  offset: number;
};

export type ShootSummaryResponse = {
  counts: { shot: number; editing: number; ready: number; delivered: number; expired: number };
  overdue: number;
};

type Mode = { server: true; cookie?: string } | { server?: false };

export function listShoots(
  params: {
    status?: string;
    overdue?: boolean;
    clientId?: string;
    q?: string;
    sort?: "due" | "created";
    limit?: number;
    offset?: number;
  },
  mode: Mode = { server: false },
): Promise<ApiResult<ShootListResponse>> {
  const sp = new URLSearchParams();
  if (params.status) sp.set("status", params.status);
  if (params.overdue !== undefined) sp.set("overdue", String(params.overdue));
  if (params.clientId) sp.set("clientId", params.clientId);
  if (params.q) sp.set("q", params.q);
  if (params.sort) sp.set("sort", params.sort);
  if (params.limit != null) sp.set("limit", String(params.limit));
  if (params.offset != null) sp.set("offset", String(params.offset));
  const q = sp.toString();
  return apiRequest<ShootListResponse>(`/api/v1/shoots${q ? `?${q}` : ""}`, { method: "GET" }, mode);
}

export function getShoot(id: string, mode: Mode = { server: false }): Promise<ApiResult<Shoot>> {
  return apiRequest<Shoot>(`/api/v1/shoots/${id}`, { method: "GET" }, mode);
}

export function getShootSummary(mode: Mode = { server: false }): Promise<ApiResult<ShootSummaryResponse>> {
  return apiRequest<ShootSummaryResponse>("/api/v1/shoots/summary", { method: "GET" }, mode);
}

export function createShoot(
  input: {
    clientId: string;
    title: string;
    shootDate?: string;
    dueDate?: string;
    priceMinor?: number;
    currency?: string;
  },
  mode: Mode = { server: false },
): Promise<ApiResult<Shoot>> {
  return apiRequest<Shoot>("/api/v1/shoots", { method: "POST", body: JSON.stringify(input) }, mode);
}

export function updateShoot(
  id: string,
  input: Partial<{ clientId: string; title: string; shootDate: string; dueDate: string; priceMinor: number; currency: string }>,
  mode: Mode = { server: false },
): Promise<ApiResult<Shoot>> {
  return apiRequest<Shoot>(`/api/v1/shoots/${id}`, { method: "PATCH", body: JSON.stringify(input) }, mode);
}

export function setShootStatus(
  id: string,
  status: "shot" | "editing" | "ready",
  mode: Mode = { server: false },
): Promise<ApiResult<Shoot>> {
  return apiRequest<Shoot>(`/api/v1/shoots/${id}/status`, { method: "POST", body: JSON.stringify({ status }) }, mode);
}

export function deleteShoot(id: string, mode: Mode = { server: false }): Promise<ApiResult<void>> {
  return apiRequest<void>(`/api/v1/shoots/${id}`, { method: "DELETE" }, mode);
}
