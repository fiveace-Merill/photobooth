import { overdueText } from "@/lib/dates";

export function OverdueLabel({ daysOverdue }: { daysOverdue: number }) {
  return <span className="text-sm font-medium text-destructive">{overdueText(daysOverdue)}</span>;
}
