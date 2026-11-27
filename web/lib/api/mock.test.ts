import { describe, expect, it } from "vitest";
import { MockDeliveryApi } from "./mock";

describe("MockDeliveryApi", () => {
  it("keeps demo and demo-paid tokens working", async () => {
    const api = new MockDeliveryApi();
    const demo = await api.getDelivery("demo");
    const paid = await api.getDelivery("demo-paid");

    expect(demo?.status).toBe("preview");
    expect(paid?.status).toBe("unlocked");
    if (demo?.status === "preview") {
      expect(demo.features.payments).toBe("mpesa");
      expect(demo.photos[0].thumbUrl).toContain("/demo/previews/");
      expect("originalUrl" in demo.photos[0]).toBe(false);
    }
    if (paid?.status === "unlocked") {
      expect(paid.downloadAllUrl).toBe("/api/v1/deliveries/demo-paid/download");
      expect(paid.photos[0].originalUrl).toContain("/api/v1/deliveries/demo-paid/photos/");
    }
  });
});
