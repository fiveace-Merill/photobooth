import { Badge } from "@/components/ui/badge";
import type { ShootStatus } from "@/lib/api/shoots";

const labels: Record<ShootStatus, string> = {
  shot: "Shot",
  editing: "Editing",
  ready: "Ready",
  delivered: "Delivered",
  expired: "Expired",
};

export function StatusChip({ status }: { status: ShootStatus }) {
  if (status === "shot") return <Badge variant="muted">Shot</Badge>;
  if (status === "editing") return <Badge variant="secondary">Editing</Badge>;
  if (status === "ready") return <Badge variant="volt">Ready</Badge>;
  if (status === "delivered") return <Badge variant="default">Delivered</Badge>;
  return <Badge variant="outline">{labels.expired}</Badge>;
}
