export type PaymentStatus = "idle" | "form" | "waiting" | "failed" | "succeeded";

export type PaymentState = {
  status: PaymentStatus;
  phone: string;
  paymentId?: string;
  error: string | null;
};

export type PaymentEvent =
  | { type: "OPEN" }
  | { type: "CLOSE" }
  | { type: "SET_PHONE"; phone: string }
  | { type: "SUBMIT"; phone: string; paymentId: string }
  | { type: "FAILED"; error: string }
  | { type: "RETRY" }
  | { type: "SUCCEEDED" };

export const initialPaymentState: PaymentState = {
  status: "idle",
  phone: "",
  error: null,
};

export function paymentReducer(
  state: PaymentState,
  event: PaymentEvent,
): PaymentState {
  switch (event.type) {
    case "OPEN":
      return { ...state, status: "form", error: null };
    case "CLOSE":
      return initialPaymentState;
    case "SET_PHONE":
      if (state.status !== "form" && state.status !== "failed") return state;
      return { ...state, phone: event.phone };
    case "SUBMIT":
      return {
        status: "waiting",
        phone: event.phone,
        paymentId: event.paymentId,
        error: null,
      };
    case "FAILED":
      if (state.status !== "waiting") return state;
      return { ...state, status: "failed", error: event.error };
    case "RETRY":
      if (state.status !== "failed") return state;
      return { ...state, status: "form", error: null };
    case "SUCCEEDED":
      if (state.status !== "waiting") return state;
      return { ...state, status: "succeeded", error: null };
    default:
      return state;
  }
}
