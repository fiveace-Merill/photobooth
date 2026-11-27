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

export type DeliveryNotification = {
  channel: "email";
  status: "queued" | "sent" | "failed";
  createdAt?: string;
  errorCode?: string | null;
  recipient?: string;
};

export type CreateDeliveryLinkResult = {
  url: string;
  expiresAt: string;
  notifications?: DeliveryNotification[];
};

export type MarkPaidInput = {
  method: "cash" | "mpesa_direct" | "other";
  note?: string;
};

export type SendDeliveryOptions = {
  send?: { email: boolean };
};

export type DeliveryMessagePreview = {
  subject: string;
  bodyText: string;
  recipientMasked: string;
  emailsRemainingToday: number;
  availability: {
    email: { available: boolean; reason?: string };
  };
};

export type ShootNotificationsResult = {
  items: Array<{
    channel: "email";
    status: "queued" | "sent" | "failed";
    createdAt: string;
    errorCode?: string | null;
    recipient: string;
  }>;
};

export function getShootDelivery(shootId: string): Promise<ApiResult<ShootDeliveryState>> {
  return apiRequest<ShootDeliveryState>(`/api/v1/shoots/${shootId}/delivery`, { method: "GET" }, { server: false });
}

export function deliverShoot(
  shootId: string,
  input: SendDeliveryOptions = {},
): Promise<ApiResult<CreateDeliveryLinkResult>> {
  return apiRequest<CreateDeliveryLinkResult>(`/api/v1/shoots/${shootId}/deliver`, { method: "POST", body: JSON.stringify(input) }, { server: false });
}

export function regenerateShootLink(
  shootId: string,
  input: SendDeliveryOptions = {},
): Promise<ApiResult<CreateDeliveryLinkResult>> {
  return apiRequest<CreateDeliveryLinkResult>(`/api/v1/shoots/${shootId}/link/regenerate`, { method: "POST", body: JSON.stringify(input) }, { server: false });
}

export function revokeShootLink(shootId: string): Promise<ApiResult<void>> {
  return apiRequest<void>(`/api/v1/shoots/${shootId}/link/revoke`, { method: "POST" }, { server: false });
}

export function markShootPaid(shootId: string, input: MarkPaidInput): Promise<ApiResult<void>> {
  return apiRequest<void>(`/api/v1/shoots/${shootId}/mark-paid`, { method: "POST", body: JSON.stringify(input) }, { server: false });
}

export function getShootNotifications(shootId: string): Promise<ApiResult<ShootNotificationsResult>> {
  return apiRequest<ShootNotificationsResult>(`/api/v1/shoots/${shootId}/notifications`, { method: "GET" }, { server: false });
}

export function getShootDeliveryMessagePreview(shootId: string): Promise<ApiResult<DeliveryMessagePreview>> {
  return apiRequest<DeliveryMessagePreview>(`/api/v1/shoots/${shootId}/delivery/message-preview`, { method: "GET" }, { server: false });
}
