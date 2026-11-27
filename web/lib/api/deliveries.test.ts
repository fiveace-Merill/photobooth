import { describe, expect, it } from "vitest";
import { messageForApiError } from "./errors";

describe("delivery email error mapping", () => {
  it("maps new delivery email codes to clear messages", () => {
    expect(messageForApiError({ status: 422, code: "channel_unavailable", error: "x" })).toContain("email");
    expect(messageForApiError({ status: 429, code: "too_many_sends", error: "x" })).toContain("Too many");
    expect(messageForApiError({ status: 429, code: "email_daily_limit_reached", error: "x" })).toContain("today");
  });
});
