import type { CompletePart } from "@/lib/api/photos";
import type { UploadCallbacks, UploadFileEntry, UploadIO, UploadManagerConfig } from "./types";

type InternalEntry = UploadFileEntry & {
  abort: AbortController;
  partProgress: Map<number, number>;
  partSizeBytes: number;
  partCount: number;
};

const DEFAULTS: Required<UploadManagerConfig> = {
  maxConcurrentFiles: 3,
  maxConcurrentParts: 3,
  maxPartAttempts: 5,
  backoffBaseMs: 500,
  backoffMaxMs: 8000,
};

function defaultSleep(ms: number, signal?: AbortSignal): Promise<void> {
  if (ms <= 0) return Promise.resolve();
  return new Promise((resolve, reject) => {
    const t = setTimeout(() => { signal?.removeEventListener("abort", onAbort); resolve(); }, ms);
    const onAbort = () => { clearTimeout(t); reject(new DOMException("Aborted", "AbortError")); };
    if (signal?.aborted) { onAbort(); return; }
    signal?.addEventListener("abort", onAbort, { once: true });
  });
}

function backoffMs(attempt: number, base: number, max: number, random: () => number): number {
  const exp = Math.min(max, base * 2 ** Math.max(0, attempt - 1));
  return Math.round(exp * (0.75 + random() * 0.5));
}

function etagOf(headers: string | null): string {
  if (!headers) return "";
  for (const line of headers.split(/\r?\n/)) {
    const i = line.indexOf(":");
    if (i === -1) continue;
    if (line.slice(0, i).trim().toLowerCase() === "etag") return line.slice(i + 1).trim().replace(/^W\//, "").replaceAll('"', "");
  }
  return "";
}

export function createXhrPut(): UploadIO["putPart"] {
  return (_url, body, contentType, signal, onProgress) => {
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      xhr.open("PUT", _url);
      xhr.upload.onprogress = (e) => { if (e.lengthComputable) onProgress?.(e.loaded); };
      xhr.onload = () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          const etag = etagOf(xhr.getAllResponseHeaders());
          if (!etag) { reject(new Error("missing_etag")); return; }
          resolve({ etag });
          return;
        }
        reject(new Error(`http_${xhr.status}`));
      };
      xhr.onerror = () => reject(new Error("network_error"));
      xhr.onabort = () => reject(new DOMException("Aborted", "AbortError"));
      signal.addEventListener("abort", () => xhr.abort(), { once: true });
      xhr.setRequestHeader("Content-Type", contentType);
      xhr.send(body);
    });
  };
}

export class UploadManager {
  private cfg: Required<UploadManagerConfig>;
  private io: Required<UploadIO>;
  private entries = new Map<string, InternalEntry>();
  private queue: string[] = [];
  private activeFiles = 0;
  private activeParts = 0;
  private callbacks: UploadCallbacks;
  private counter = 0;

  constructor(io: UploadIO, cfg: UploadManagerConfig = {}, callbacks: UploadCallbacks = {}) {
    this.cfg = { ...DEFAULTS, ...cfg };
    this.io = {
      now: () => Date.now(),
      sleep: defaultSleep,
      random: () => Math.random(),
      ...io,
    } as Required<UploadIO>;
    this.callbacks = callbacks;
  }

  getEntries(): UploadFileEntry[] {
    return Array.from(this.entries.values()).map((e) => ({
      localId: e.localId,
      file: e.file,
      filename: e.filename,
      sizeBytes: e.sizeBytes,
      contentType: e.contentType,
      status: e.status,
      progress: e.progress,
      uploadedBytes: e.uploadedBytes,
      photoId: e.photoId,
      error: e.error,
      attempts: e.attempts,
    }));
  }

  hasActiveUploads(): boolean {
    return this.getEntries().some((e) => e.status === "waiting" || e.status === "uploading");
  }

  enqueue(files: File[]): string[] {
    const ids: string[] = [];
    for (const file of files) {
      const id = `up_${++this.counter}_${file.name}_${file.size}`;
      const entry: InternalEntry = {
        localId: id,
        file,
        filename: file.name,
        sizeBytes: file.size,
        contentType: file.type,
        status: "waiting",
        progress: 0,
        uploadedBytes: 0,
        attempts: 0,
        abort: new AbortController(),
        partProgress: new Map(),
        partSizeBytes: 0,
        partCount: 0,
      };
      this.entries.set(id, entry);
      this.queue.push(id);
      ids.push(id);
    }
    this.emit();
    this.drain();
    return ids;
  }

  cancel(localId: string): void {
    const e = this.entries.get(localId);
    if (!e) return;
    if (e.status === "ready" || e.status === "failed" || e.status === "cancelled") return;
    e.abort.abort();
    e.status = "cancelled";
    e.error = "Upload cancelled.";
    this.emit();
    if (e.photoId) void this.io.deletePhoto(e.photoId).catch(() => {});
  }

  retry(localId: string): void {
    const e = this.entries.get(localId);
    if (!e || (e.status !== "failed" && e.status !== "cancelled")) return;
    e.abort = new AbortController();
    e.status = "waiting";
    e.error = undefined;
    e.progress = 0;
    e.uploadedBytes = 0;
    e.partProgress.clear();
    e.photoId = undefined;
    e.attempts = 0;
    this.queue.push(localId);
    this.emit();
    this.drain();
  }

  private emit() { this.callbacks.onChange?.(this.getEntries()); }

  private drain() {
    while (this.activeFiles < this.cfg.maxConcurrentFiles && this.queue.length > 0) {
      const id = this.queue.shift();
      if (!id) break;
      const e = this.entries.get(id);
      if (!e || e.status !== "waiting") continue;
      this.activeFiles++;
      void this.uploadFile(e).finally(() => {
        this.activeFiles--;
        this.drain();
      });
    }
  }

  private async uploadFile(e: InternalEntry): Promise<void> {
    try {
      e.status = "uploading";
      e.attempts++;
      this.emit();
      const created = await this.io.createUpload({
        filename: e.filename,
        contentType: e.contentType,
        sizeBytes: e.sizeBytes,
      });
      if (e.abort.signal.aborted) throw new DOMException("Aborted", "AbortError");
      e.photoId = created.photoId;
      e.partSizeBytes = created.partSizeBytes;
      e.partCount = created.partCount;
      this.emit();

      const etags = await this.uploadAllParts(e, created.uploadParts);
      if (e.abort.signal.aborted) throw new DOMException("Aborted", "AbortError");
      const parts: CompletePart[] = etags
        .map((etag, i) => ({ partNumber: i + 1, etag }))
        .sort((a, b) => a.partNumber - b.partNumber);
      await this.io.completeUpload(created.photoId, parts);
      e.status = "processing";
      e.progress = 1;
      e.uploadedBytes = e.sizeBytes;
      this.emit();
    } catch (err) {
      if (e.abort.signal.aborted || (err instanceof DOMException && err.name === "AbortError")) {
        if (e.status !== "cancelled") e.status = "cancelled";
        this.emit();
        return;
      }
      e.status = "failed";
      e.error = err instanceof Error ? err.message : "Upload failed.";
      this.emit();
    }
  }

  private async uploadAllParts(e: InternalEntry, initialParts: { partNumber: number; url: string }[]): Promise<string[]> {
    const urlByPart = new Map<number, string>();
    for (const p of initialParts) urlByPart.set(p.partNumber, p.url);
    const etags: string[] = new Array(e.partCount).fill("");
    let next = 1;
    let active = 0;
    let failed: unknown = null;

    await new Promise<void>((resolve) => {
      const maybeDone = () => {
        if (failed) { if (active === 0) resolve(); return; }
        if (next > e.partCount && active === 0) { resolve(); return; }
        while (!failed && active < this.cfg.maxConcurrentParts && next <= e.partCount && this.activeParts < this.cfg.maxConcurrentParts) {
          const partNumber = next++;
          active++;
          this.activeParts++;
          void this.uploadPartWithRetry(e, partNumber, urlByPart)
            .then((etag) => { etags[partNumber - 1] = etag; })
            .catch((err) => { failed = failed ?? err; })
            .finally(() => { active--; this.activeParts--; maybeDone(); });
        }
      };
      maybeDone();
    });
    if (failed) throw failed;
    return etags;
  }

  private async uploadPartWithRetry(
    e: InternalEntry,
    partNumber: number,
    urlByPart: Map<number, string>,
  ): Promise<string> {
    let attempt = 0;
    while (attempt < this.cfg.maxPartAttempts) {
      attempt++;
      if (e.abort.signal.aborted) throw new DOMException("Aborted", "AbortError");
      let url = urlByPart.get(partNumber);
      if (!url) {
        const fresh = await this.io.presignParts(e.photoId!, [partNumber]);
        url = fresh.find((p) => p.partNumber === partNumber)?.url;
        if (url) urlByPart.set(partNumber, url);
      }
      if (!url) throw new Error("missing_part_url");

      const start = (partNumber - 1) * e.partSizeBytes;
      const end = Math.min(start + e.partSizeBytes, e.sizeBytes);
      const blob = e.file.slice(start, end);
      try {
        const { etag } = await this.io.putPart(url, blob, e.contentType, e.abort.signal, (loaded) => {
          e.partProgress.set(partNumber, Math.min(loaded, blob.size));
          this.recomputeProgress(e);
        });
        e.partProgress.set(partNumber, blob.size);
        this.recomputeProgress(e);
        return etag;
      } catch (err) {
        if (e.abort.signal.aborted || (err instanceof DOMException && err.name === "AbortError")) throw err;
        urlByPart.delete(partNumber);
        try {
          const fresh = await this.io.presignParts(e.photoId!, [partNumber]);
          const nextUrl = fresh.find((p) => p.partNumber === partNumber)?.url;
          if (nextUrl) urlByPart.set(partNumber, nextUrl);
        } catch {}
        if (attempt >= this.cfg.maxPartAttempts) throw err;
        const delay = backoffMs(attempt, this.cfg.backoffBaseMs, this.cfg.backoffMaxMs, this.io.random);
        await this.io.sleep(delay, e.abort.signal);
      }
    }
    throw new Error("upload_part_failed");
  }

  private recomputeProgress(e: InternalEntry) {
    let uploaded = 0;
    for (const v of e.partProgress.values()) uploaded += v;
    e.uploadedBytes = Math.min(uploaded, e.sizeBytes);
    e.progress = e.sizeBytes > 0 ? e.uploadedBytes / e.sizeBytes : 0;
    this.emit();
  }
}
