import { describe, expect, it } from "vitest";
import { isTypedConfirmationMatch } from "./confirm";

describe("isTypedConfirmationMatch", () => {
  it("matches exact expected value", () => {
    expect(isTypedConfirmationMatch("DELETE", "DELETE")).toBe(true);
  });

  it("trims surrounding whitespace", () => {
    expect(isTypedConfirmationMatch("  DELETE  ", "DELETE")).toBe(true);
  });

  it("rejects wrong value", () => {
    expect(isTypedConfirmationMatch("delete", "DELETE")).toBe(false);
    expect(isTypedConfirmationMatch("", "DELETE")).toBe(false);
  });
});
