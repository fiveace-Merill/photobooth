export type WaitlistStatus = "new" | "invited" | "declined" | "spam";
export type InvitationStatus = "pending" | "used" | "revoked" | "expired";
export type PhotographerStatus = "active" | "suspended";

export type StatusTone = "muted" | "secondary" | "volt" | "default" | "outline" | "destructive";

export type StatusChip = { label: string; variant: StatusTone };

const waitlist: Record<WaitlistStatus, StatusChip> = {
  new: { label: "New", variant: "volt" },
  invited: { label: "Invited", variant: "default" },
  declined: { label: "Declined", variant: "muted" },
  spam: { label: "Spam", variant: "destructive" },
};

const invitations: Record<InvitationStatus, StatusChip> = {
  pending: { label: "Pending", variant: "volt" },
  used: { label: "Used", variant: "default" },
  revoked: { label: "Revoked", variant: "muted" },
  expired: { label: "Expired", variant: "outline" },
};

const photographers: Record<PhotographerStatus, StatusChip> = {
  active: { label: "Active", variant: "default" },
  suspended: { label: "Suspended", variant: "destructive" },
};

export function waitlistStatusChip(status: string): StatusChip {
  return waitlist[status as WaitlistStatus] ?? { label: status || "Unknown", variant: "outline" };
}

export function invitationStatusChip(status: string): StatusChip {
  return invitations[status as InvitationStatus] ?? { label: status || "Unknown", variant: "outline" };
}

export function photographerStatusChip(status: string): StatusChip {
  return photographers[status as PhotographerStatus] ?? { label: status || "Unknown", variant: "outline" };
}
