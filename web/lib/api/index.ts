import { MockDeliveryApi } from "./mock";
import type { DeliveryApi } from "./client";

// Swap this export to a real fetch-based client later. The rest of the app
// depends on the DeliveryApi interface only.
export const deliveryApi: DeliveryApi = new MockDeliveryApi();
