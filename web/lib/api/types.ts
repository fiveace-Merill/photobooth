// Shared delivery types. Money is integer minor units (KES cents), never floats.

export type DeliveryPhoto = {
  id: string;
  width: number;
  height: number;
  thumbUrl: string;
  previewUrl: string;
};

export type PreviewDeliveryPhoto = DeliveryPhoto;

export type UnlockedDeliveryPhoto = DeliveryPhoto & {
  originalUrl: string;
};

export type DeliveryFeatures = {
  payments: "manual" | "mpesa";
  reviews: boolean;
};

type DeliveryBase = {
  studioName: string;
  studioPhone: string | null;
  title: string;
  shotOn: string;
  priceMinor: number;
  currency: "KES";
  expiresAt: string;
  features: DeliveryFeatures;
};

// SECURITY RULE: PreviewDelivery must NOT contain any original or download URLs.
// The browser must never receive originals before payment is verified server-side.
export type PreviewDelivery = DeliveryBase & {
  status: "preview";
  photos: PreviewDeliveryPhoto[];
};

export type UnlockedDelivery = DeliveryBase & {
  status: "unlocked";
  downloadAllUrl: string;
  totalOriginalBytes: number;
  photos: UnlockedDeliveryPhoto[];
};

export type Delivery = PreviewDelivery | UnlockedDelivery;

export type PaymentStatus = "pending" | "succeeded" | "failed";

export type StartPaymentResult = {
  paymentId: string;
  status: PaymentStatus;
};

export type PaymentStatusResult = {
  paymentId: string;
  status: PaymentStatus;
};
