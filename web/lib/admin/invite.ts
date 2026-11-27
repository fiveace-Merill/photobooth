export type WaitlistInviteEntry = {
  contactEmail: string;
  contactPhone: string;
};

export type InviteDialogState = {
  email: string;
  needsEmail: boolean;
  canSendEmail: boolean;
  note: string | null;
};

export function getInviteDialogState(entry: WaitlistInviteEntry, emailConfigured: boolean): InviteDialogState {
  const email = (entry.contactEmail || "").trim();
  const phoneOnly = !email && (entry.contactPhone || "").trim().length > 0;
  return {
    email,
    needsEmail: phoneOnly,
    canSendEmail: emailConfigured,
    note: phoneOnly ? "This person left a phone number only. Ask them on WhatsApp for an email address before inviting." : null,
  };
}

export function canSubmitInvite(input: { email: string; planId: string; days: number }): boolean {
  return input.email.trim().length > 0 && input.planId.trim().length > 0 && Number.isFinite(input.days) && input.days > 0;
}
