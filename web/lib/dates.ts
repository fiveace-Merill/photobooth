const NAIROBI = "Africa/Nairobi";

function nairobiToday(now = new Date()): string {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: NAIROBI,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(now);
  const y = parts.find((p) => p.type === "year")?.value ?? "1970";
  const m = parts.find((p) => p.type === "month")?.value ?? "01";
  const d = parts.find((p) => p.type === "day")?.value ?? "01";
  return `${y}-${m}-${d}`;
}

export function formatDisplayDate(isoDate: string): string {
  const [y, m, d] = isoDate.split("-").map(Number);
  const date = new Date(Date.UTC(y, (m ?? 1) - 1, d ?? 1));
  return new Intl.DateTimeFormat("en-GB", {
    day: "2-digit",
    month: "short",
    year: "numeric",
    timeZone: "UTC",
  }).format(date);
}

export function overdueText(daysOverdue: number): string {
  return `Overdue by ${daysOverdue} day${daysOverdue === 1 ? "" : "s"}`;
}

export function dueDateLabel(
  dueDate: string | null | undefined,
  overdue: boolean,
  daysOverdue: number,
  now = new Date(),
): string {
  if (!dueDate) return "No due date";
  if (overdue) return overdueText(daysOverdue);
  if (dueDate === nairobiToday(now)) return "Due today";
  return `Due ${formatDisplayDate(dueDate)}`;
}

export function formatShootDate(isoDate: string | null | undefined): string {
  if (!isoDate) return "No shoot date";
  return formatDisplayDate(isoDate);
}
