import { describe, expect, it } from "vitest";
import { formatKesInput, formatMoney, parseKesToMinor } from "./money";

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

describe("parseKesToMinor", () => {
  it("parses whole KES values with commas to minor units", () => {
    expect(parseKesToMinor("3,000")).toBe(300000);
    expect(parseKesToMinor("3000")).toBe(300000);
  });

  it("parses zero", () => {
    expect(parseKesToMinor("0")).toBe(0);
  });

  it("rejects invalid or negative values", () => {
    expect(() => parseKesToMinor("")).toThrow();
    expect(() => parseKesToMinor("-1")).toThrow();
    expect(() => parseKesToMinor("abc")).toThrow();
  });
});

describe("formatKesInput", () => {
  it("formats minor units as whole-KES input string", () => {
    expect(formatKesInput(300000)).toBe("3,000");
    expect(formatKesInput(0)).toBe("0");
  });
});
