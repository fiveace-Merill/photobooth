import { describe, expect, it } from "vitest";
import { messageForAdminError } from "./errors";

describe("messageForAdminError", () => {
  it("maps known admin error codes", () => {
    expect(messageForAdminError({ status: 401, code: "invalid_credentials", error: "x" })).toMatch(/invalid/i);
    expect(messageForAdminError({ status: 429, code: "rate_limited", error: "x" })).toMatch(/too many/i);
    expect(messageForAdminError({ status: 401, code: "unauthorized", error: "x" })).toMatch(/session/i);
    expect(messageForAdminError({ status: 404, code: "not_found", error: "x" })).toMatch(/could not be found/i);
    expect(messageForAdminError({ status: 400, code: "invalid_input", error: "x" })).toMatch(/check/i);
    expect(messageForAdminError({ status: 400, code: "invalid_json", error: "x" })).toMatch(/invalid/i);
    expect(messageForAdminError({ status: 403, code: "invalid_origin", error: "x" })).toMatch(/could not be sent/i);
    expect(messageForAdminError({ status: 415, code: "invalid_content_type", error: "x" })).toMatch(/could not be sent/i);
    expect(messageForAdminError({ status: 500, code: "internal", error: "x" })).toMatch(/wrong/i);
  });

  it("falls back safely", () => {
    expect(messageForAdminError({ status: 418, code: "teapot", error: "x" })).toMatch(/wrong/i);
  });
});
