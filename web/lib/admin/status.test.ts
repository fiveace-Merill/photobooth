import { describe, expect, it } from "vitest";
import { invitationStatusChip, photographerStatusChip, waitlistStatusChip } from "./status";

describe("admin status chips", () => {
  it("maps waitlist statuses", () => {
    expect(waitlistStatusChip("new")).toEqual({ label: "New", variant: "volt" });
    expect(waitlistStatusChip("invited")).toEqual({ label: "Invited", variant: "default" });
    expect(waitlistStatusChip("declined")).toEqual({ label: "Declined", variant: "muted" });
    expect(waitlistStatusChip("spam")).toEqual({ label: "Spam", variant: "destructive" });
  });

  it("maps invitation statuses", () => {
    expect(invitationStatusChip("pending")).toEqual({ label: "Pending", variant: "volt" });
    expect(invitationStatusChip("used")).toEqual({ label: "Used", variant: "default" });
    expect(invitationStatusChip("revoked")).toEqual({ label: "Revoked", variant: "muted" });
    expect(invitationStatusChip("expired")).toEqual({ label: "Expired", variant: "outline" });
  });

  it("maps photographer statuses", () => {
    expect(photographerStatusChip("active")).toEqual({ label: "Active", variant: "default" });
    expect(photographerStatusChip("suspended")).toEqual({ label: "Suspended", variant: "destructive" });
  });

  it("falls back with text label", () => {
    expect(waitlistStatusChip("weird").label).toBe("weird");
  });
});
