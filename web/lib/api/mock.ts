import type { DeliveryApi } from "./client";
import type {
  Delivery,
  PaymentStatus,
  PaymentStatusResult,
  PreviewDeliveryPhoto,
  StartPaymentResult,
  UnlockedDeliveryPhoto,
} from "./types";
import { normalizeKenyanPhone } from "../phone";

type MockPayment = {
  token: string;
  phone: string;
  status: PaymentStatus;
  createdAt: number;
};

const payments = new Map<string, MockPayment>();

const demoFiles = [
  { file: "aleksandra-sapozhnikova.jpg", width: 800, height: 1200, originalBytes: 1803518 },
  { file: "bryan-heng.jpg", width: 1200, height: 801, originalBytes: 2406630 },
  { file: "eren-yildiz.jpg", width: 750, height: 1200, originalBytes: 1250930 },
  { file: "modern-dairy-portraits.jpg", width: 1200, height: 800, originalBytes: 8084825 },
  { file: "samuel-boyer.jpg", width: 1200, height: 800, originalBytes: 4173168 },
  { file: "sarmat-batagov.jpg", width: 960, height: 1200, originalBytes: 1883144 },
  { file: "vaclav-pechar-lakeside.jpg", width: 1200, height: 674, originalBytes: 3085981 },
  { file: "vaclav-pechar-coast.jpg", width: 800, height: 1200, originalBytes: 3242975 },
] as const;

function baseDelivery() {
  return {
    studioName: "Wanjiru Kamau Photography",
    studioPhone: "254712345678",
    title: "Njeri and Kamau’s wedding",
    shotOn: "2026-11-14",
    priceMinor: 300000,
    currency: "KES" as const,
    expiresAt: "2026-12-22T00:00:00Z",
    features: { payments: "mpesa" as const, reviews: true },
  };
}

function makePreviewPhotos(): PreviewDeliveryPhoto[] {
  return demoFiles.map((file, i) => ({
    id: `photo-${i + 1}`,
    width: file.width,
    height: file.height,
    thumbUrl: `/demo/previews/${file.file}`,
    previewUrl: `/demo/previews/${file.file}`,
  }));
}

function makeUnlockedPhotos(token: string): UnlockedDeliveryPhoto[] {
  return demoFiles.map((file, i) => ({
    id: `photo-${i + 1}`,
    width: file.width,
    height: file.height,
    thumbUrl: `/demo/previews/${file.file}`,
    previewUrl: `/demo/previews/${file.file}`,
    originalUrl: `/api/v1/deliveries/${token}/photos/photo-${i + 1}/original`,
  }));
}

export function totalOriginalBytes(): number {
  return demoFiles.reduce((sum, f) => sum + f.originalBytes, 0);
}

export class MockDeliveryApi implements DeliveryApi {
  async getDelivery(token: string): Promise<Delivery | null> {
    if (token === "demo") {
      return { status: "preview", ...baseDelivery(), photos: makePreviewPhotos() };
    }
    if (token === "demo-paid") {
      return {
        status: "unlocked",
        ...baseDelivery(),
        downloadAllUrl: `/api/v1/deliveries/demo-paid/download`,
        totalOriginalBytes: totalOriginalBytes(),
        photos: makeUnlockedPhotos("demo-paid"),
      };
    }
    return null;
  }

  async startPayment(token: string, phone: string): Promise<StartPaymentResult> {
    const normalized = normalizeKenyanPhone(phone);
    const paymentId = `pay_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
    payments.set(paymentId, { token, phone: normalized, status: "pending", createdAt: Date.now() });
    return { paymentId, status: "pending" };
  }

  async getPaymentStatus(token: string, paymentId: string): Promise<PaymentStatusResult> {
    const payment = payments.get(paymentId);
    if (!payment || payment.token !== token) throw new Error("payment not found");
    if (payment.status === "pending") {
      const elapsed = Date.now() - payment.createdAt;
      if (elapsed >= 3000) payment.status = payment.phone.endsWith("000") ? "failed" : "succeeded";
    }
    return { paymentId, status: payment.status };
  }

  async submitReview(token: string, rating: number, comment: string): Promise<{ ok: true }> {
    void token;
    void rating;
    void comment;
    return { ok: true };
  }
}
