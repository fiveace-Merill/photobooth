import { describe, expect, it, vi } from "vitest";
import { UploadManager } from "./upload-manager";
import { validateFiles } from "./validate";
import type { UploadIO } from "./types";

function makeFile(name: string, size: number, type = "image/jpeg"): File {
  const bytes = new Uint8Array(size);
  for (let i = 0; i < size; i++) bytes[i] = i % 251;
  return new File([bytes], name, { type });
}

function deferred<T>() {
  let resolve!: (v: T | PromiseLike<T>) => void;
  let reject!: (e?: unknown) => void;
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej; });
  return { promise, resolve, reject };
}

function makeIO(overrides: Partial<UploadIO> = {}): UploadIO {
  let photoSeq = 0;
  return {
    createUpload: vi.fn(async ({ sizeBytes }) => {
      photoSeq++;
      const partCount = Math.max(1, Math.ceil(sizeBytes / 10));
      return {
        photoId: `photo_${photoSeq}`,
        shootId: "shoot_1",
        partSizeBytes: 10,
        partCount,
        uploadParts: Array.from({ length: Math.min(partCount, 3) }, (_: unknown, i: number) => ({ partNumber: i + 1, url: `https://upload/${photoSeq}/${i + 1}`, expiresAt: new Date().toISOString() })),
        status: "uploading",
      };
    }),
    presignParts: vi.fn(async (_photoId: string, partNumbers: number[]) => partNumbers.map((n: number) => ({ partNumber: n, url: `https://upload/refreshed/${n}`, expiresAt: new Date().toISOString() }))),
    completeUpload: vi.fn(async (photoId) => ({ id: photoId, shootId: "shoot_1", filename: "f.jpg", contentType: "image/jpeg", sizeBytes: 1, status: "processing", createdAt: "", updatedAt: "", sortKey: 0 })),
    deletePhoto: vi.fn(async () => {}),
    putPart: vi.fn(async (_url, body) => ({ etag: `etag-${body.size}` })),
    sleep: vi.fn(async () => {}),
    random: () => 0.5,
    ...overrides,
  };
}

describe("validateFiles", () => {
  it("rejects bad type/oversize/quota/count and accepts valid files", () => {
    const res = validateFiles([
      makeFile("a.txt", 10, "text/plain"),
      makeFile("big.jpg", 2000),
      makeFile("ok1.jpg", 100),
      makeFile("ok2.jpg", 100),
      makeFile("noquota.jpg", 100),
    ], { allowedTypes: ["image/jpeg", "image/png", "image/webp"], maxBytes: 1024, remainingQuotaBytes: 150, remainingPhotoCount: 2 });
    expect(res.accepted.map((f) => f.name)).toEqual(["ok1.jpg"]);
    expect(res.rejected).toHaveLength(4);
    expect(res.rejected[0].reason).toContain("unsupported file type");
    expect(res.rejected[1].reason).toContain("too large");
    expect(res.rejected[3].reason).toContain("not enough storage quota");
  });
});

describe("UploadManager", () => {
  it("slices parts, collects ETags, and completes", async () => {
    const file = makeFile("photo.jpg", 25);
    const io = makeIO();
    const mgr = new UploadManager(io, { maxConcurrentFiles: 1, maxConcurrentParts: 2 });
    mgr.enqueue([file]);
    await vi.waitFor(() => expect(io.completeUpload).toHaveBeenCalledTimes(1));
    expect(io.createUpload).toHaveBeenCalledWith({ filename: "photo.jpg", contentType: "image/jpeg", sizeBytes: 25 });
    expect(io.putPart).toHaveBeenCalledTimes(3);
    const parts = (io.completeUpload as ReturnType<typeof vi.fn>).mock.calls[0][1] as Array<{partNumber:number;etag:string}>;
    expect(parts).toEqual([
      { partNumber: 1, etag: "etag-10" },
      { partNumber: 2, etag: "etag-10" },
      { partNumber: 3, etag: "etag-5" },
    ]);
    expect(mgr.getEntries()[0].status).toBe("processing");
  });

  it("retries part failure with refreshed URL and backoff", async () => {
    const file = makeFile("photo.jpg", 10);
    let calls = 0;
    const io = makeIO({
      putPart: vi.fn(async (_url, body) => {
        calls++;
        if (calls === 1) throw new Error("network_error");
        return { etag: `etag-${body.size}` };
      }),
    });
    const mgr = new UploadManager(io, { maxConcurrentFiles: 1, maxConcurrentParts: 1, backoffBaseMs: 10, backoffMaxMs: 10 });
    mgr.enqueue([file]);
    await vi.waitFor(() => expect(io.completeUpload).toHaveBeenCalledTimes(1));
    expect(io.presignParts).toHaveBeenCalled();
    expect(io.sleep).toHaveBeenCalled();
  });

  it("fails after max part attempts", async () => {
    const file = makeFile("photo.jpg", 10);
    const io = makeIO({ putPart: vi.fn(async () => { throw new Error("network_error"); }) });
    const mgr = new UploadManager(io, { maxConcurrentFiles: 1, maxConcurrentParts: 1, maxPartAttempts: 3, backoffBaseMs: 1, backoffMaxMs: 1 });
    mgr.enqueue([file]);
    await vi.waitFor(() => expect(mgr.getEntries()[0].status).toBe("failed"));
    expect((io.putPart as ReturnType<typeof vi.fn>).mock.calls.length).toBe(3);
  });

  it("respects file and part concurrency limits", async () => {
    const files = [makeFile("a.jpg", 10), makeFile("b.jpg", 10), makeFile("c.jpg", 10), makeFile("d.jpg", 10)];
    let activeFiles = 0; let maxActiveFiles = 0;
    let activeParts = 0; let maxActiveParts = 0;
    const gates: Array<ReturnType<typeof deferred<void>>> = [];
    const io = makeIO({
      createUpload: vi.fn(async ({ filename }) => {
        activeFiles++; maxActiveFiles = Math.max(maxActiveFiles, activeFiles);
        return { photoId: filename, shootId: "shoot_1", partSizeBytes: 10, partCount: 1, uploadParts: [{ partNumber: 1, url: `https://u/${filename}/1`, expiresAt: "" }], status: "uploading" };
      }),
      putPart: vi.fn(async (_url, body) => {
        activeParts++; maxActiveParts = Math.max(maxActiveParts, activeParts);
        const g = deferred<void>(); gates.push(g); await g.promise;
        activeParts--;
        return { etag: `etag-${body.size}` };
      }),
      completeUpload: vi.fn(async (photoId) => { activeFiles--; return { id: photoId, shootId: "shoot_1", filename: "f", contentType: "image/jpeg", sizeBytes: 1, status: "processing", createdAt: "", updatedAt: "", sortKey: 0 }; }),
    });
    const mgr = new UploadManager(io, { maxConcurrentFiles: 2, maxConcurrentParts: 2 });
    mgr.enqueue(files);
    await vi.waitFor(() => expect(gates.length).toBe(2));
    expect(maxActiveFiles).toBe(2);
    expect(maxActiveParts).toBeLessThanOrEqual(2);
    gates.forEach((g) => g.resolve());
    await vi.waitFor(() => expect(gates.length).toBe(4));
    gates.forEach((g) => g.resolve());
    await vi.waitFor(() => expect(io.completeUpload).toHaveBeenCalledTimes(4));
  });

  it("cancels upload and calls delete endpoint", async () => {
    const file = makeFile("photo.jpg", 30);
    const blocked = deferred<{ etag: string }>();
    const io = makeIO({
      createUpload: vi.fn(async () => ({ photoId: "photo_1", shootId: "shoot_1", partSizeBytes: 10, partCount: 3, uploadParts: [1, 2, 3].map((n) => ({ partNumber: n, url: `u${n}`, expiresAt: "" })), status: "uploading" })),
      putPart: vi.fn(() => blocked.promise),
    });
    const mgr = new UploadManager(io, { maxConcurrentFiles: 1, maxConcurrentParts: 1 });
    const [id] = mgr.enqueue([file]);
    await vi.waitFor(() => expect(io.putPart).toHaveBeenCalledTimes(1));
    mgr.cancel(id);
    blocked.reject(new DOMException("Aborted", "AbortError"));
    await vi.waitFor(() => expect(mgr.getEntries()[0].status).toBe("cancelled"));
    await vi.waitFor(() => expect(io.deletePhoto).toHaveBeenCalledWith("photo_1"));
  });
});
