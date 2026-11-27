"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { toast } from "sonner";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { PhotoUploader } from "./photo-uploader";
import { UploadRow } from "./upload-row";
import { PhotoGallery } from "./photo-gallery";
import { createPhotoUpload, completePhotoUpload, deletePhoto, listPhotos, presignPhotoParts, retryPhoto, type Photo, type PhotoListResult } from "@/lib/api/photos";
import { messageForApiError } from "@/lib/api/errors";
import { UploadManager, createXhrPut, DEFAULT_ALLOWED_TYPES, formatBytes, validateFiles, type UploadFileEntry } from "@/lib/upload";

const MAX_PHOTO_BYTES = 60 * 1024 * 1024;
const MAX_PHOTOS_PER_SHOOT = 500;
const POLL_MS = 3000;

type Props = {
  shootId: string;
  shootExpired?: boolean;
};

function usedPercent(used: number, quota: number): number {
  if (quota <= 0) return 0;
  return Math.max(0, Math.min(100, (used / quota) * 100));
}

export function ShootPhotosPanel({ shootId, shootExpired }: Props) {
  const [photos, setPhotos] = useState<Photo[]>([]);
  const [totals, setTotals] = useState({ count: 0, bytes: 0 });
  const [quota, setQuota] = useState({ usedBytes: 0, quotaBytes: 0 });
  const [uploadEntries, setUploadEntries] = useState<UploadFileEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [busyIds, setBusyIds] = useState<Set<string>>(new Set());
  const [announcement, setAnnouncement] = useState("");
  const imageRetryRef = useRef<Set<string>>(new Set());
  const pollRef = useRef<number | null>(null);

  const refresh = useCallback(async () => {
    const res = await listPhotos(shootId, { limit: 500, offset: 0 });
    if (!res.ok) {
      setLoadError(messageForApiError(res));
      setLoading(false);
      return;
    }
    const data = res.data as PhotoListResult;
    setPhotos(data.items);
    setTotals(data.totals);
    setQuota(data.quota);
    setLoadError(null);
    setLoading(false);
  }, [shootId]);

  useEffect(() => { const t = window.setTimeout(() => { void refresh(); }, 0); return () => window.clearTimeout(t); }, [refresh]);

  const manager = useMemo(() => {
    return new UploadManager({
      createUpload: async (input) => {
        const res = await createPhotoUpload(shootId, input);
        if (!res.ok) throw new Error(messageForApiError(res));
        return res.data;
      },
      presignParts: async (photoId, partNumbers) => {
        const res = await presignPhotoParts(shootId, photoId, partNumbers);
        if (!res.ok) throw new Error(messageForApiError(res));
        return res.data.parts;
      },
      completeUpload: async (photoId, parts) => {
        const res = await completePhotoUpload(shootId, photoId, parts);
        if (!res.ok) throw new Error(messageForApiError(res));
        return res.data;
      },
      deletePhoto: async (photoId) => { await deletePhoto(shootId, photoId); },
      putPart: createXhrPut(),
    }, { maxConcurrentFiles: 3, maxConcurrentParts: 3, maxPartAttempts: 5 }, {
      onChange: (entries) => {
        setUploadEntries(entries);
        const active = entries.find((e) => e.status === "uploading");
        if (active) setAnnouncement(`${active.filename}: ${Math.round(active.progress * 100)}% uploaded`);
      },
    });
  }, [shootId]);

  const hasActiveUploads = uploadEntries.some((e) => e.status === "waiting" || e.status === "uploading");
  const hasProcessing = photos.some((p) => p.status === "processing") || uploadEntries.some((e) => e.status === "processing");

  useEffect(() => {
    if (!hasActiveUploads) return;
    const handler = (e: BeforeUnloadEvent) => { e.preventDefault(); e.returnValue = ""; };
    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, [hasActiveUploads]);

  useEffect(() => {
    if (!hasProcessing) { if (pollRef.current) { window.clearInterval(pollRef.current); pollRef.current = null; } return; }
    if (pollRef.current) return;
    pollRef.current = window.setInterval(() => { void refresh(); }, POLL_MS);
    return () => { if (pollRef.current) { window.clearInterval(pollRef.current); pollRef.current = null; } };
  }, [hasProcessing, refresh]);

  useEffect(() => {
    if (!uploadEntries.some((e) => e.status === "processing")) return;
    const t = window.setTimeout(() => { void refresh(); }, 0);
    return () => window.clearTimeout(t);
  }, [uploadEntries, refresh]);

  const onFiles = (files: File[]) => {
    const remainingCount = Math.max(0, MAX_PHOTOS_PER_SHOOT - totals.count - uploadEntries.filter((e) => e.status === "waiting" || e.status === "uploading" || e.status === "processing").length);
    const { accepted, rejected } = validateFiles(files, {
      allowedTypes: DEFAULT_ALLOWED_TYPES,
      maxBytes: MAX_PHOTO_BYTES,
      remainingQuotaBytes: Math.max(0, quota.quotaBytes - quota.usedBytes - uploadEntries.filter((e) => ["waiting", "uploading", "processing"].includes(e.status)).reduce((a, e) => a + e.sizeBytes, 0)),
      remainingPhotoCount: remainingCount,
    });
    for (const r of rejected) toast.error(r.reason);
    if (accepted.length > 0) manager.enqueue(accepted);
  };

  const busy = (id: string, value: boolean) => setBusyIds((prev) => { const next = new Set(prev); if (value) next.add(id); else next.delete(id); return next; });

  const onDelete = async (ids: string[]) => {
    for (const id of ids) {
      busy(id, true);
      const res = await deletePhoto(shootId, id);
      busy(id, false);
      if (!res.ok) { toast.error(messageForApiError(res)); continue; }
    }
    setSelectedIds((prev) => { const next = new Set(prev); ids.forEach((id) => next.delete(id)); return next; });
    await refresh();
  };

  const onRetryPhoto = async (photo: Photo) => {
    busy(photo.id, true);
    const res = await retryPhoto(shootId, photo.id);
    busy(photo.id, false);
    if (!res.ok) { toast.error(messageForApiError(res)); return; }
    await refresh();
  };

  const onImageError = async (photo: Photo) => {
    if (imageRetryRef.current.has(photo.id)) return;
    imageRetryRef.current.add(photo.id);
    await refresh();
  };

  const percent = usedPercent(quota.usedBytes, quota.quotaBytes);
  const quotaWarning = percent >= 100 ? "Storage quota is full. Delete photos or upgrade your plan before uploading more." : percent >= 90 ? "You're nearing your storage quota." : null;

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Photos</CardTitle>
        <p className="text-sm text-muted-foreground">{totals.count} photos, {formatBytes(totals.bytes)}</p>
      </CardHeader>
      <CardContent className="space-y-6">
        <div className="space-y-2">
          <div className="flex items-center justify-between text-xs text-muted-foreground">
            <span>{formatBytes(quota.usedBytes)} of {formatBytes(quota.quotaBytes || 1)} used</span>
            <span>{Math.round(percent)}%</span>
          </div>
          <div className="h-2 w-full rounded-full bg-muted" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(percent)} aria-label="Storage quota used">
            <div className={percent >= 100 ? "h-2 rounded-full bg-destructive" : "h-2 rounded-full bg-primary"} style={{ width: `${percent}%` }} />
          </div>
          {quotaWarning ? <p className="text-xs text-muted-foreground" role="status">{quotaWarning}</p> : null}
        </div>

        <PhotoUploader disabled={shootExpired} onFiles={onFiles} />
        <p className="sr-only" aria-live="polite">{announcement}</p>

        {uploadEntries.length > 0 ? (
          <div className="space-y-2">
            {uploadEntries.map((entry) => (
              <UploadRow key={entry.localId} entry={entry} onCancel={(id) => manager.cancel(id)} onRetry={(id) => manager.retry(id)} />
            ))}
          </div>
        ) : null}

        {loadError ? <p className="text-sm text-destructive" role="status">{loadError}</p> : null}
        {loading ? <p className="text-sm text-muted-foreground">Loading photos…</p> : null}

        {!loading ? (
          <PhotoGallery
            photos={photos}
            selectedIds={selectedIds}
            onToggle={(id) => setSelectedIds((prev) => { const next = new Set(prev); if (next.has(id)) next.delete(id); else next.add(id); return next; })}
            onSelectAll={() => setSelectedIds(new Set(photos.map((p) => p.id)))}
            onClearSelection={() => setSelectedIds(new Set())}
            onDelete={onDelete}
            onRetry={onRetryPhoto}
            onDiscard={(photo) => void onDelete([photo.id])}
            onImageError={onImageError}
            busyIds={busyIds}
          />
        ) : null}

        <Card className="border-dashed">
          <CardHeader><CardTitle className="text-sm">Delivery</CardTitle></CardHeader>
          <CardContent><p className="text-sm text-muted-foreground">Delivery link and client access arrive in the next release.</p></CardContent>
        </Card>
      </CardContent>
    </Card>
  );
}
