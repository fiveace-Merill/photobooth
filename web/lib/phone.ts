// Kenyan Safaricom mobile numbers accepted by the prototype and Daraja STK push.
// Accepts: 07xxxxxxxx, 01xxxxxxxx, +2547xxxxxxxx, 2547xxxxxxxx (and 01 variants).
// Normalized output is 2547XXXXXXXX / 2541XXXXXXXX, the format Daraja expects.
const SAFARICOM_RE = /^(?:\+?254|0)?([71]\d{8})$/;

export function normalizeKenyanPhone(input: string): string {
  const cleaned = input.replace(/[\s-]/g, "");
  const match = cleaned.match(SAFARICOM_RE);
  if (!match) {
    throw new Error("invalid Kenyan Safaricom phone number");
  }
  return `254${match[1]}`;
}

export function validateKenyanPhone(input: string): boolean {
  try {
    normalizeKenyanPhone(input);
    return true;
  } catch {
    return false;
  }
}
