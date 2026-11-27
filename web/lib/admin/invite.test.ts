import { describe, expect, it } from "vitest";
import { canSubmitInvite, getInviteDialogState } from "./invite";

describe("getInviteDialogState", () => {
  it("prefills email when known", () => {
    const state = getInviteDialogState({ contactEmail: "a@b.com", contactPhone: "" }, true);
    expect(state.email).toBe("a@b.com");
    expect(state.needsEmail).toBe(false);
    expect(state.note).toBeNull();
    expect(state.canSendEmail).toBe(true);
  });

  it("requires email for phone-only entries", () => {
    const state = getInviteDialogState({ contactEmail: "", contactPhone: "254700000000" }, false);
    expect(state.needsEmail).toBe(true);
    expect(state.email).toBe("");
    expect(state.note).toMatch(/WhatsApp/i);
    expect(state.canSendEmail).toBe(false);
  });
});

describe("canSubmitInvite", () => {
  it("requires email, plan and valid days", () => {
    expect(canSubmitInvite({ email: "a@b.com", planId: "free", days: 7 })).toBe(true);
    expect(canSubmitInvite({ email: "", planId: "free", days: 7 })).toBe(false);
    expect(canSubmitInvite({ email: "a@b.com", planId: "", days: 7 })).toBe(false);
    expect(canSubmitInvite({ email: "a@b.com", planId: "free", days: 0 })).toBe(false);
  });
});
