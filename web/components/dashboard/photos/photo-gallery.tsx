"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Skeleton } from "@/components/ui/skeleton";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import type { Photo } from "@/lib/api/photos";
import { formatBytes } from "@/lib/upload/validate";

type Props = {
  photos: Photo[];
  selectedIds: Set<string>;
  onToggle: (id: string) => void;
  onSelectAll: () => void;
  onClearSelection: () => void;
  onDelete: (ids: string[]) => void;
  onRetry: (photo: Photo) => void;
  onDiscard: (photo: Photo) => void;
  onImageError: (photo: Photo) => void;
  busyIds: Set<string>;
};

function stateLabel(photo: Photo): string {
  if (photo.status === "processing") return "Preparing preview";
  if (photo.status === "uploading") return "Incomplete";
  if (photo.status === "failed") return photo.error ? `Failed: ${photo.error}` : "Failed";
  return photo.status;
}

export function PhotoGallery({ photos, selectedIds, onToggle, onSelectAll, onClearSelection, onDelete, onRetry, onDiscard, onImageError, busyIds }: Props) {
  const [confirmIds, setConfirmIds] = useState<string[] | null>(null);
  const selectedCount = selectedIds.size;
  const allSelected = photos.length > 0 && selectedCount === photos.length;

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <Button type="button" variant="outline" className="min-h-11" onClick={allSelected ? onClearSelection : onSelectAll} disabled={photos.length === 0}>
          {allSelected ? "Clear selection" : "Select all"}
        </Button>
        <Button type="button" variant="destructive" className="min-h-11" disabled={selectedCount === 0} onClick={() => setConfirmIds(Array.from(selectedIds))}>
          Delete selected{selectedCount > 0 ? ` (${selectedCount})` : ""}
        </Button>
        <span className="text-xs text-muted-foreground" aria-live="polite">{selectedCount > 0 ? `${selectedCount} selected` : ""}</span>
      </div>

      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
        {photos.map((photo) => {
          const busy = busyIds.has(photo.id);
          const checked = selectedIds.has(photo.id);
          return (
            <div key={photo.id} className="rounded-lg border border-border bg-card p-2">
              <div className="mb-2 flex items-center justify-between gap-2">
                <label className="flex min-h-11 min-w-11 items-center justify-center rounded-md">
                  <Checkbox checked={checked} onCheckedChange={() => onToggle(photo.id)} aria-label={`Select ${photo.filename}`} />
                </label>
                <Button type="button" variant="ghost" className="min-h-11 px-3 text-xs" disabled={busy} onClick={() => setConfirmIds([photo.id])}>Delete</Button>
              </div>

              <div className="aspect-square overflow-hidden rounded-md bg-muted">
                {photo.status === "ready" && photo.thumbUrl ? (
                  // Plain img: presigned R2 thumb URLs are short-lived signed URLs; next/image remotePatterns are not practical for dynamic signed hosts.
                  <img
                    src={photo.thumbUrl}
                    alt={photo.filename}
                    width={photo.width ?? 480}
                    height={photo.height ?? 480}
                    loading="lazy"
                    decoding="async"
                    className="h-full w-full object-cover"
                    onError={() => onImageError(photo)}
                  />
                ) : photo.status === "processing" ? (
                  <div className="flex h-full flex-col items-center justify-center gap-2 p-3">
                    <Skeleton className="h-full w-full rounded-md" />
                    <span className="sr-only">Preparing preview</span>
                  </div>
                ) : (
                  <div className="flex h-full items-center justify-center p-3 text-center text-xs text-muted-foreground">
                    {photo.status === "uploading" ? "Incomplete upload" : photo.status === "failed" ? "Preview unavailable" : "No preview"}
                  </div>
                )}
              </div>

              <div className="mt-2 space-y-1">
                <p className="truncate text-xs font-medium" title={photo.filename}>{photo.filename}</p>
                <p className="text-xs text-muted-foreground">{formatBytes(photo.sizeBytes)}</p>
                <p className="text-xs text-muted-foreground" aria-live="polite">{stateLabel(photo)}</p>
              </div>

              <div className="mt-2 flex gap-2">
                {photo.status === "failed" ? <Button type="button" variant="outline" className="min-h-11 flex-1" disabled={busy} onClick={() => onRetry(photo)}>Retry</Button> : null}
                {photo.status === "uploading" ? <Button type="button" variant="outline" className="min-h-11 flex-1" disabled={busy} onClick={() => onDiscard(photo)}>Discard</Button> : null}
              </div>
            </div>
          );
        })}
      </div>

      <AlertDialog open={confirmIds !== null} onOpenChange={(open) => { if (!open) setConfirmIds(null); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete {confirmIds?.length === 1 ? "photo" : "photos"}?</AlertDialogTitle>
            <AlertDialogDescription>
              This will permanently delete {confirmIds?.length === 1 ? "this photo" : `${confirmIds?.length ?? 0} photos`}. This cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel className="min-h-11">Cancel</AlertDialogCancel>
            <AlertDialogAction className="min-h-11 bg-destructive text-white hover:bg-destructive/90" onClick={() => { if (confirmIds) onDelete(confirmIds); setConfirmIds(null); }}>Delete</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
