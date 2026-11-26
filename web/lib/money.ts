// Money is stored as integer minor units (KES cents), never floats.
// Formatting happens at the UI edge.
export function formatMoney(minor: number, currency: string): string {
  if (!Number.isInteger(minor)) {
    throw new Error("minor units must be an integer");
  }
  const sign = minor < 0 ? "-" : "";
  const abs = Math.abs(minor);
  const whole = Math.floor(abs / 100);
  const cents = abs % 100;
  const wholeFormatted = whole.toLocaleString("en-KE");
  if (cents === 0) {
    return `${sign}${currency} ${wholeFormatted}`;
  }
  return `${sign}${currency} ${wholeFormatted}.${String(cents).padStart(2, "0")}`;
}

// Parse a whole-KES user input like "3,000" or "3000" into minor units.
export function parseKesToMinor(input: string): number {
  const cleaned = input.replace(/,/g, "").trim();
  if (!/^\d+$/.test(cleaned)) {
    throw new Error("invalid KES amount");
  }
  const whole = Number(cleaned);
  if (!Number.isSafeInteger(whole) || whole < 0 || whole > 1_000_000) {
    throw new Error("invalid KES amount");
  }
  return whole * 100;
}

export function formatKesInput(minor: number): string {
  if (!Number.isInteger(minor) || minor < 0) {
    throw new Error("minor units must be a non-negative integer");
  }
  const whole = Math.floor(minor / 100);
  return whole.toLocaleString("en-KE");
}
