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
