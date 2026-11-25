import { describe, expect, it } from "vitest";
import { normalizeKenyanPhone, validateKenyanPhone } from "./phone";

describe("Kenyan Safaricom phone validation", () => {
  it("accepts local 07xx numbers", () => {
    expect(validateKenyanPhone("0712 345 678")).toBe(true);
    expect(normalizeKenyanPhone("0712 345 678")).toBe("254712345678");
  });

  it("accepts local 01xx numbers", () => {
    expect(validateKenyanPhone("0112345678")).toBe(true);
    expect(normalizeKenyanPhone("0112345678")).toBe("254112345678");
  });

  it("accepts +254 and 254 formats", () => {
    expect(normalizeKenyanPhone("+254712345678")).toBe("254712345678");
    expect(normalizeKenyanPhone("254712345678")).toBe("254712345678");
  });

  it("rejects invalid numbers", () => {
    expect(validateKenyanPhone("")).toBe(false);
    expect(validateKenyanPhone("12345")).toBe(false);
    expect(validateKenyanPhone("+255712345678")).toBe(false);
  });
});
