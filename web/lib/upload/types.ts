import type { CompletePart, CreateUploadResult, Photo, UploadPartURL } from "@/lib/api/photos";

export type UploadStatus =
  | "waiting"
  | "uploading"
  | "processing"
  | "ready"
  | "failed"
  | "cancelled"
  | "incomplete";

export type UploadFileEntry = {
  localId: string;
  file: File;
  filename: string;
  sizeBytes: number;
  contentType: string;
  status: UploadStatus;
  progress: number; // 0..1
  uploadedBytes: number;
  photoId?: string;
  error?: string;
  attempts: number;
};

export type UploadValidationConfig = {
  allowedTypes: readonly string[];
  maxBytes: number;
  remainingQuotaBytes: number;
  remainingPhotoCount: number;
};

export type UploadValidationResult = {
  accepted: File[];
  rejected: Array<{ file: File; reason: string }>;
};

export type PutPartResult = { etag: string };

export type UploadIO = {
  createUpload: (input: { filename: string; contentType: string; sizeBytes: number }) => Promise<CreateUploadResult>;
  presignParts: (photoId: string, partNumbers: number[]) => Promise<UploadPartURL[]>;
  completeUpload: (photoId: string, parts: CompletePart[]) => Promise<Photo>;
  deletePhoto: (photoId: string) => Promise<void>;
  putPart: (
    url: string,
    body: Blob,
    contentType: string,
    signal: AbortSignal,
    onProgress?: (loaded: number) => void,
  ) => Promise<PutPartResult>;
  now?: () => number;
  sleep?: (ms: number, signal?: AbortSignal) => Promise<void>;
  random?: () => number;
};

export type UploadManagerConfig = {
  maxConcurrentFiles?: number;
  maxConcurrentParts?: number;
  maxPartAttempts?: number;
  backoffBaseMs?: number;
  backoffMaxMs?: number;
};

export type UploadCallbacks = {
  onChange?: (entries: UploadFileEntry[]) => void;
};
