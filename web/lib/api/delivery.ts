import type { DeliveryApi } from "./client";
import type { Delivery } from "./types";

function baseUrl(): string {
  if (typeof window !== "undefined") return "";
  return process.env.API_URL ?? process.env.API_ORIGIN ?? "http://localhost:8080";
}

export class RealDeliveryApi implements DeliveryApi {
  async getDelivery(token: string): Promise<Delivery | null> {
    const res = await fetch(`${baseUrl()}/api/v1/deliveries/${encodeURIComponent(token)}`, {
      method: "GET",
      cache: "no-store",
      headers: { Accept: "application/json" },
    });
    if (res.status === 404) return null;
    if (!res.ok) throw new Error(`delivery fetch failed: ${res.status}`);
    return (await res.json()) as Delivery;
  }
}
