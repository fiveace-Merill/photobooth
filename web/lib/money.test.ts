import { describe, expect, it } from "vitest";
import { formatMoney } from "./money";

describe("formatMoney", () => {
  it("formats KES cents as whole currency amount", () => {
    expect(formatMoney(300000, "KES")).toBe("KES 3,000");
  });

  it("formats zero", () => {
    expect(formatMoney(0, "KES")).toBe("KES 0");
  });

  it("formats partial amounts with cents when needed", () => {
    expect(formatMoney(125050, "KES")).toBe("KES 1,250.50");
  });
});
