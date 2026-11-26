import { describe, expect, it } from "vitest";
import { dueDateLabel, formatDisplayDate, overdueText } from "./dates";

describe("date helpers (Africa/Nairobi)", () => {
  it("formats due date labels", () => {
    expect(formatDisplayDate("2026-11-26")).toBe("26 Nov 2026");
  });

  it("returns due today label when not overdue", () => {
    expect(dueDateLabel("2026-11-26", false, 0, new Date("2026-11-26T15:00:00+03:00"))).toBe("Due today");
  });

  it("returns overdue text with days", () => {
    expect(overdueText(3)).toBe("Overdue by 3 days");
    expect(overdueText(1)).toBe("Overdue by 1 day");
  });

  it("returns upcoming/normal due label", () => {
    expect(dueDateLabel("2026-11-30", false, 0, new Date("2026-11-26T09:00:00+03:00"))).toBe("Due 30 Nov 2026");
  });
});
