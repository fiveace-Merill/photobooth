import { describe, expect, it } from "vitest";
import { messageForApiCode } from "./errors";

describe("messageForApiCode", () => {
  it("maps all docs/api.md error codes", () => {
    expect(messageForApiCode("invalid_json")).toMatch(/invalid/i);
    expect(messageForApiCode("invalid_input")).toMatch(/check/i);
    expect(messageForApiCode("unauthorized")).toMatch(/session/i);
    expect(messageForApiCode("not_found")).toMatch(/could not be found/i);
    expect(messageForApiCode("client_not_found")).toMatch(/client/i);
    expect(messageForApiCode("client_phone_exists")).toMatch(/phone/i);
    expect(messageForApiCode("client_has_shoots")).toMatch(/shoots/i);
    expect(messageForApiCode("shoot_not_deletable")).toMatch(/deleted/i);
    expect(messageForApiCode("invalid_status_transition")).toMatch(/status/i);
    expect(messageForApiCode("invalid_content_type")).toMatch(/request/i);
    expect(messageForApiCode("invalid_origin")).toMatch(/request/i);
    expect(messageForApiCode("signup_closed")).toMatch(/invite/i);
    expect(messageForApiCode("invite_invalid")).toMatch(/invitation/i);
    expect(messageForApiCode("internal")).toMatch(/wrong/i);
  });

  it("falls back safely", () => {
    expect(messageForApiCode(undefined)).toMatch(/wrong/i);
  });
});
