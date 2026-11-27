import type { DeliveryApi } from "./client";
import { MockDeliveryApi } from "./mock";
import { RealDeliveryApi } from "./delivery";

const mock = new MockDeliveryApi();
const real = new RealDeliveryApi();

export function getDeliveryApi(token: string): DeliveryApi {
  if (token === "demo" || token === "demo-paid") return mock;
  if (process.env.USE_MOCK_DELIVERY === "true" || process.env.NEXT_PUBLIC_USE_MOCK_DELIVERY === "true") return mock;
  return real;
}

export const deliveryApi: DeliveryApi = mock;
