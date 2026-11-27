import { describe, expect, it } from "vitest";
import { messageForApiError } from "./errors";

function emailAvailabilityReason(reason?: string) {
  if (!reason) return "Client email is unavailable.";
  if (reason === "too_many_sends") return "Too many delivery emails were sent for this shoot recently. Try again later.";
  if (reason === "email_daily_limit_reached") return "You've reached today's email limit. Try again tomorrow.";
  return "Client email is unavailable. Add or fix their email address.";
}

describe("delivery email UI mapping", () => {
  it("maps backend error codes", () => {
    expect(messageForApiError({ status: 422, code: "channel_unavailable", error: "x" })).toMatch(/email/i);
    expect(messageForApiError({ status: 429, code: "too_many_sends", error: "x" })).toMatch(/too many/i);
    expect(messageForApiError({ status: 429, code: "email_daily_limit_reached", error: "x" })).toMatch(/today/i);
    expect(messageForApiError({ status: 503, code: "email_unavailable", error: "x" })).toMatch(/configured/i);
  });

  it("maps availability reasons", () => {
    expect(emailAvailabilityReason("too_many_sends")).toMatch(/recently/i);
    expect(emailAvailabilityReason("email_daily_limit_reached")).toMatch(/tomorrow/i);
    expect(emailAvailabilityReason("channel_unavailable")).toMatch(/fix their email/i);
  });
});
