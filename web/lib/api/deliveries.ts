import { apiRequest } from "./request";
import type { ApiResult } from "./errors";

export type DeliveryLinkState = "active" | "revoked" | "none";

export type ShootDeliveryEvent = {
  id: string;
  shootId: string;
  photoId?: string | null;
  kind: string;
  payload: Record<string, unknown>;
  createdAt: string;
};

export type ShootDeliveryState = {
  linkState: DeliveryLinkState;
  expiresAt?: string;
  lastViewedAt?: string;
  unlocked: boolean;
  events: ShootDeliveryEvent[];
};

export type CreateDeliveryLinkResult = {
  url: string;
  expiresAt: string;
};

export type MarkPaidInput = {
  method: "cash" | "mpesa_direct" | "other";
  note?: string;
};

export function getShootDelivery(shootId: string): Promise<ApiResult<ShootDeliveryState>> {
  return apiRequest<ShootDeliveryState>(`/api/v1/shoots/${shootId}/delivery`, { method: "GET" }, { server: false });
}

export function deliverShoot(shootId: string): Promise<ApiResult<CreateDeliveryLinkResult>> {
  return apiRequest<CreateDeliveryLinkResult>(`/api/v1/shoots/${shootId}/deliver`, { method: "POST" }, { server: false });
}

export function regenerateShootLink(shootId: string): Promise<ApiResult<CreateDeliveryLinkResult>> {
  return apiRequest<CreateDeliveryLinkResult>(`/api/v1/shoots/${shootId}/link/regenerate`, { method: "POST" }, { server: false });
}

export function revokeShootLink(shootId: string): Promise<ApiResult<void>> {
  return apiRequest<void>(`/api/v1/shoots/${shootId}/link/revoke`, { method: "POST" }, { server: false });
}

export function markShootPaid(shootId: string, input: MarkPaidInput): Promise<ApiResult<void>> {
  return apiRequest<void>(`/api/v1/shoots/${shootId}/mark-paid`, { method: "POST", body: JSON.stringify(input) }, { server: false });
}
