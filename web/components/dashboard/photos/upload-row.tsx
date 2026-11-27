"use client";

import { Button } from "@/components/ui/button";
import { formatBytes } from "@/lib/upload/validate";
import type { UploadFileEntry } from "@/lib/upload/types";

type Props = {
  entry: UploadFileEntry;
  onCancel: (id: string) => void;
  onRetry: (id: string) => void;
};

function statusLabel(status: UploadFileEntry["status"]): string {
  switch (status) {
    case "waiting": return "Waiting";
    case "uploading": return "Uploading";
    case "processing": return "Processing";
    case "ready": return "Ready";
    case "failed": return "Failed";
    case "cancelled": return "Cancelled";
    case "incomplete": return "Incomplete";
    default: return status;
  }
}

export function UploadRow({ entry, onCancel, onRetry }: Props) {
  const pct = Math.round(Math.max(0, Math.min(1, entry.progress)) * 100);
  const showProgress = entry.status === "uploading" || entry.status === "waiting";
  const canCancel = entry.status === "uploading" || entry.status === "waiting";
  const canRetry = entry.status === "failed" || entry.status === "cancelled";
  return (
    <div className="rounded-lg border border-border p-3">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <p className="truncate text-sm font-medium">{entry.filename}</p>
            <span className="shrink-0 text-xs text-muted-foreground">{formatBytes(entry.sizeBytes)}</span>
          </div>
          <p className="text-xs text-muted-foreground">{statusLabel(entry.status)}{entry.error ? ` — ${entry.error}` : ""}</p>
          {showProgress ? (
            <div className="mt-2 space-y-1">
              <div className="h-2 w-full rounded-full bg-muted" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={pct} aria-label={`${entry.filename} upload progress`}>
                <div className="h-2 rounded-full bg-primary transition-[width]" style={{ width: `${pct}%` }} />
              </div>
              <p className="text-xs text-muted-foreground" aria-live="polite">{pct}%</p>
            </div>
          ) : null}
          {entry.status === "processing" ? <p className="mt-2 text-xs text-muted-foreground" aria-live="polite">Processing preview…</p> : null}
        </div>
        <div className="flex shrink-0 gap-2">
          {canRetry ? <Button type="button" variant="outline" className="min-h-11" onClick={() => onRetry(entry.localId)}>Retry</Button> : null}
          {canCancel ? <Button type="button" variant="outline" className="min-h-11" onClick={() => onCancel(entry.localId)}>Cancel</Button> : null}
        </div>
      </div>
    </div>
  );
}
