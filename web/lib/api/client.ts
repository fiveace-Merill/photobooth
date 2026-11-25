import type { Delivery, PaymentStatusResult, StartPaymentResult } from "./types";

export interface DeliveryApi {
  getDelivery(token: string): Promise<Delivery | null>;
  startPayment(token: string, phone: string): Promise<StartPaymentResult>;
  getPaymentStatus(token: string, paymentId: string): Promise<PaymentStatusResult>;
  submitReview(
    token: string,
    rating: number,
    comment: string,
  ): Promise<{ ok: true }>;
}
