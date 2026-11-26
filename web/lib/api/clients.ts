import { apiRequest } from "./request";
import type { ApiResult } from "./errors";

export type Client = {
  id: string;
  name: string;
  phone?: string;
  email?: string;
  notes?: string;
  createdAt: string;
  updatedAt: string;
};

export type ClientListResponse = {
  items: Client[];
  total: number;
  limit: number;
  offset: number;
};

type Mode = { server: true; cookie?: string } | { server?: false };

export function listClients(
  params: { q?: string; limit?: number; offset?: number },
  mode: Mode = { server: false },
): Promise<ApiResult<ClientListResponse>> {
  const sp = new URLSearchParams();
  if (params.q) sp.set("q", params.q);
  if (params.limit != null) sp.set("limit", String(params.limit));
  if (params.offset != null) sp.set("offset", String(params.offset));
  const q = sp.toString();
  return apiRequest<ClientListResponse>(`/api/v1/clients${q ? `?${q}` : ""}`, { method: "GET" }, mode);
}

export function getClient(id: string, mode: Mode = { server: false }): Promise<ApiResult<Client>> {
  return apiRequest<Client>(`/api/v1/clients/${id}`, { method: "GET" }, mode);
}

export function createClient(
  input: { name: string; phone?: string; email?: string; notes?: string },
  mode: Mode = { server: false },
): Promise<ApiResult<Client>> {
  return apiRequest<Client>("/api/v1/clients", { method: "POST", body: JSON.stringify(input) }, mode);
}

export function updateClient(
  id: string,
  input: Partial<{ name: string; phone: string; email: string; notes: string }>,
  mode: Mode = { server: false },
): Promise<ApiResult<Client>> {
  return apiRequest<Client>(`/api/v1/clients/${id}`, { method: "PATCH", body: JSON.stringify(input) }, mode);
}

export function deleteClient(id: string, mode: Mode = { server: false }): Promise<ApiResult<void>> {
  return apiRequest<void>(`/api/v1/clients/${id}`, { method: "DELETE" }, mode);
}
