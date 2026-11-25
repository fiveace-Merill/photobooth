import { describe, expect, it } from "vitest";
import {
  initialPaymentState,
  paymentReducer,
} from "./payment-machine";

describe("paymentReducer", () => {
  it("opens and closes the form", () => {
    const opened = paymentReducer(initialPaymentState, { type: "OPEN" });
    expect(opened.status).toBe("form");

    const closed = paymentReducer(opened, { type: "CLOSE" });
    expect(closed).toEqual(initialPaymentState);
  });

  it("moves to waiting on submit", () => {
    const waiting = paymentReducer(
      { status: "form", phone: "0712345678", error: null },
      { type: "SUBMIT", phone: "0712345678", paymentId: "pay_1" },
    );
    expect(waiting).toMatchObject({
      status: "waiting",
      phone: "0712345678",
      paymentId: "pay_1",
    });
  });

  it("handles failure and retry", () => {
    const failed = paymentReducer(
      { status: "waiting", phone: "0712000000", paymentId: "pay_1", error: null },
      { type: "FAILED", error: "Payment didn’t go through" },
    );
    expect(failed.status).toBe("failed");

    const retried = paymentReducer(failed, { type: "RETRY" });
    expect(retried).toMatchObject({ status: "form", phone: "0712000000", error: null });
  });

  it("handles success", () => {
    const succeeded = paymentReducer(
      { status: "waiting", phone: "0712345678", paymentId: "pay_1", error: null },
      { type: "SUCCEEDED" },
    );
    expect(succeeded.status).toBe("succeeded");
  });
});
