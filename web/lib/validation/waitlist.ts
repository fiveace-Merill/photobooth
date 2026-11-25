import { normalizeKenyanPhone, validateKenyanPhone } from "@/lib/phone";

export type WaitlistInput = {
  name: string;
  contact: string;
  photographyType: string;
  shootsPerMonth: string;
  consent: boolean;
  website: string; // honeypot
};

export type ValidWaitlistInput = {
  name: string;
  contact: string;
  normalizedContact: string;
  contactType: "email" | "phone";
  photographyType: string;
  shootsPerMonth: string;
  consent: true;
};

export type WaitlistValidationResult =
  | { ok: true; data: ValidWaitlistInput }
  | { ok: false; silent: true }
  | { ok: false; silent: false; errors: Partial<Record<keyof WaitlistInput, string>> };

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export function validateWaitlistInput(input: WaitlistInput): WaitlistValidationResult {
  // Honeypot: bots often fill hidden fields. Reject silently so the bot gets no signal.
  if (input.website.trim() !== "") {
    return { ok: false, silent: true };
  }

  const errors: Partial<Record<keyof WaitlistInput, string>> = {};
  const name = input.name.trim();
  const contact = input.contact.trim();

  if (!name) errors.name = "Enter your name.";
  if (!contact) {
    errors.contact = "Enter an email address or phone number.";
  }

  let contactType: "email" | "phone" | null = null;
  let normalizedContact = contact;
  if (contact) {
    if (EMAIL_RE.test(contact)) {
      contactType = "email";
      normalizedContact = contact.toLowerCase();
    } else if (validateKenyanPhone(contact)) {
      contactType = "phone";
      normalizedContact = normalizeKenyanPhone(contact);
    } else {
      errors.contact = "Enter a valid email address or Kenyan phone number.";
    }
  }

  if (!input.photographyType) errors.photographyType = "Select a type of photography.";
  if (!input.shootsPerMonth) errors.shootsPerMonth = "Select how many shoots you do.";
  if (input.consent !== true) errors.consent = "You need to agree before we can contact you.";

  if (Object.keys(errors).length > 0 || !contactType) {
    return { ok: false, silent: false, errors };
  }

  return {
    ok: true,
    data: {
      name,
      contact,
      normalizedContact,
      contactType,
      photographyType: input.photographyType,
      shootsPerMonth: input.shootsPerMonth,
      consent: true,
    },
  };
}
