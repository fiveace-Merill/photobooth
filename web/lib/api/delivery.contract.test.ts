import { describe, expect, it } from "vitest";
import type { Delivery } from "./types";

describe("delivery contract", () => {
  it("maps locked delivery without original fields", () => {
    const payload = {
      status: "preview",
      studioName: "Studio",
      studioPhone: "254712345678",
      title: "Shoot",
      shotOn: "2026-11-20",
      priceMinor: 300000,
      currency: "KES",
      expiresAt: "2026-12-20T00:00:00Z",
      features: { payments: "manual", reviews: false },
      photos: [{ id: "p1", width: 100, height: 80, thumbUrl: "/t", previewUrl: "/p" }],
    } satisfies Delivery;

    const json = JSON.stringify(payload);
    expect(json).not.toContain('"originalUrl"');
    expect(json).not.toContain('/original');
    expect(payload.features.payments).toBe("manual");
  });

  it("maps unlocked delivery with API original paths", () => {
    const payload = {
      status: "unlocked",
      studioName: "Studio",
      studioPhone: null,
      title: "Shoot",
      shotOn: "2026-11-20",
      priceMinor: 0,
      currency: "KES",
      expiresAt: "2026-12-20T00:00:00Z",
      features: { payments: "manual", reviews: false },
      downloadAllUrl: "/api/v1/deliveries/tok/download",
      totalOriginalBytes: 123,
      photos: [{ id: "p1", width: 100, height: 80, thumbUrl: "/t", previewUrl: "/p", originalUrl: "/api/v1/deliveries/tok/photos/p1/original" }],
    } satisfies Delivery;

    expect(payload.photos[0].originalUrl).toContain("/api/v1/deliveries/");
  });
});
