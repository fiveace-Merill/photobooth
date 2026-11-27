import type { UploadValidationConfig, UploadValidationResult } from "./types";

export const DEFAULT_ALLOWED_TYPES = ["image/jpeg", "image/png", "image/webp"] as const;

export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const units = ["KB", "MB", "GB"];
  let value = bytes / 1024;
  let unit = units[0];
  for (let i = 1; i < units.length && value >= 1024; i++) {
    value /= 1024;
    unit = units[i];
  }
  const digits = value >= 10 || unit === "GB" ? 0 : 1;
  return `${value.toFixed(digits)} ${unit}`;
}

export function validateFiles(files: File[], cfg: UploadValidationConfig): UploadValidationResult {
  const accepted: File[] = [];
  const rejected: Array<{ file: File; reason: string }> = [];
  let quotaLeft = cfg.remainingQuotaBytes;
  let countLeft = cfg.remainingPhotoCount;

  for (const file of files) {
    if (!cfg.allowedTypes.includes(file.type)) {
      rejected.push({ file, reason: `${file.name}: unsupported file type. Use JPEG, PNG, or WebP.` });
      continue;
    }
    if (file.size <= 0 || file.size > cfg.maxBytes) {
      rejected.push({ file, reason: `${file.name}: file is too large (max ${formatBytes(cfg.maxBytes)}).` });
      continue;
    }
    if (countLeft <= 0) {
      rejected.push({ file, reason: `${file.name}: this shoot can't accept more photos.` });
      continue;
    }
    if (file.size > quotaLeft) {
      rejected.push({ file, reason: `${file.name}: not enough storage quota remaining.` });
      continue;
    }
    accepted.push(file);
    quotaLeft -= file.size;
    countLeft -= 1;
  }
  return { accepted, rejected };
}
