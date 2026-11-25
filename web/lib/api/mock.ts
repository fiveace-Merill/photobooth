import type { DeliveryApi } from "./client";
import type {
  Delivery,
  PaymentStatus,
  PaymentStatusResult,
  StartPaymentResult,
  UnlockedPhoto,
} from "./types";
import { normalizeKenyanPhone } from "../phone";

type MockPayment = {
  token: string;
  phone: string;
  status: PaymentStatus;
  createdAt: number;
};

const payments = new Map<string, MockPayment>();

const photos = Array.from({ length: 12 }, (_, i) => {
  const ratios = [0.75, 1.33, 1, 0.67, 1.5, 0.75, 1, 1.33, 0.67, 0.75, 1.5, 1];
  const r = ratios[i] ?? 1;
  const width = 1200;
  const height = Math.round(width / r);
  return {
    id: `photo-${i + 1}`,
    width,
    height,
    previewUrl: null,
  };
});

function baseDelivery() {
  return {
    studioName: "Wanjiru Kamau Photography",
    title: "Njeri and Kamau’s wedding",
    shotOn: "14 November 2026",
    priceMinor: 300000,
    currency: "KES" as const,
    expiresAt: "22 December 2026",
    photos,
  };
}

function makeUnlockedPhotos(): UnlockedPhoto[] {
  return photos.map((p) => ({
    ...p,
    originalUrl: `/mock/originals/${p.id}.jpg`,
  }));
}

export class MockDeliveryApi implements DeliveryApi {
  async getDelivery(token: string): Promise<Delivery | null> {
    if (token === "demo") {
      return { status: "preview", ...baseDelivery() };
    }
    if (token === "demo-paid") {
      return {
        status: "unlocked",
        ...baseDelivery(),
        downloadAllUrl: "/mock/downloads/demo-paid.zip",
        totalOriginalBytes: 96 * 1024 * 1024,
        photos: makeUnlockedPhotos(),
      };
    }
    return null;
  }

  async startPayment(token: string, phone: string): Promise<StartPaymentResult> {
    const normalized = normalizeKenyanPhone(phone);
    const paymentId = `pay_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
    payments.set(paymentId, {
      token,
      phone: normalized,
      status: "pending",
      createdAt: Date.now(),
    });
    return { paymentId, status: "pending" };
  }

  async getPaymentStatus(token: string, paymentId: string): Promise<PaymentStatusResult> {
    const payment = payments.get(paymentId);
    if (!payment || payment.token !== token) {
      throw new Error("payment not found");
    }
    // Mock M-PESA callback timing: pending briefly, then succeed/fail.
    // A phone ending in 000 makes the failure path easy to test.
    if (payment.status === "pending") {
      const elapsed = Date.now() - payment.createdAt;
      if (elapsed >= 3000) {
        payment.status = payment.phone.endsWith("000") ? "failed" : "succeeded";
      }
    }
    return { paymentId, status: payment.status };
  }

  async submitReview(
    token: string,
    rating: number,
    comment: string,
  ): Promise<{ ok: true }> {
    void token;
    void rating;
    void comment;
    return { ok: true };
  }
}
