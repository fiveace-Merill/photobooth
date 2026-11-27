import { describe, expect, it } from "vitest";
import { csvFilename } from "./csv";

describe("csvFilename", () => {
  it("includes prefix and csv extension", () => {
    expect(csvFilename("waitlist")).toMatch(/^waitlist-\d{4}-\d{2}-\d{2}\.csv$/);
  });
});
