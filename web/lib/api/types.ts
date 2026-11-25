// Shared delivery types. Money is integer minor units (KES cents), never floats.

export type Photo = {
  id: string;
  width: number;
  height: number;
  previewUrl: string | null;
};

export type PreviewPhoto = Photo;

export type UnlockedPhoto = Photo & {
  originalUrl: string;
};

type DeliveryBase = {
  studioName: string;
  title: string;
  shotOn: string;
  priceMinor: number;
  currency: "KES";
  expiresAt: string;
};

// SECURITY RULE: PreviewDelivery must NOT contain any original or download URLs.
// The browser must never receive originals before payment is verified server-side.
export type PreviewDelivery = DeliveryBase & {
  status: "preview";
  photos: PreviewPhoto[];
};

export type UnlockedDelivery = DeliveryBase & {
  status: "unlocked";
  downloadAllUrl: string;
  totalOriginalBytes: number;
  photos: UnlockedPhoto[];
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
