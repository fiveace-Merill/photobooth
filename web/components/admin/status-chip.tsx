import { Badge } from "@/components/ui/badge";
import type { StatusChip } from "@/lib/admin/status";

export function AdminStatusChip({ chip }: { chip: StatusChip }) {
  return <Badge variant={chip.variant}>{chip.label}</Badge>;
}
