import { apiRequest } from "./request";
import type { ApiResult } from "./errors";

export type PhotoStatus = "uploading" | "processing" | "ready" | "failed";

export type Photo = {
  id: string;
  shootId: string;
  filename: string;
  contentType: string;
  sizeBytes: number;
  width?: number;
  height?: number;
  status: PhotoStatus | string;
  error?: string;
  processedAt?: string;
  createdAt: string;
  updatedAt: string;
  sortKey: number;
  thumbUrl?: string;
};

export type PhotoListResult = {
  items: Photo[];
  totals: { count: number; bytes: number };
  quota: { usedBytes: number; quotaBytes: number };
};

export type UploadPartURL = {
  partNumber: number;
  url: string;
  expiresAt: string;
};

export type CreateUploadResult = {
  photoId: string;
  shootId: string;
  partSizeBytes: number;
  partCount: number;
  uploadParts: UploadPartURL[];
  status: string;
};

export type CompletePart = { partNumber: number; etag: string };

type Mode = { server: true; cookie?: string } | { server?: false };

export function createPhotoUpload(
  shootId: string,
  input: { filename: string; contentType: string; sizeBytes: number },
  mode: Mode = { server: false },
): Promise<ApiResult<CreateUploadResult>> {
  return apiRequest<CreateUploadResult>(
    `/api/v1/shoots/${shootId}/photos/uploads`,
    { method: "POST", body: JSON.stringify(input) },
    mode,
  );
}

export function presignPhotoParts(
  shootId: string,
  photoId: string,
  partNumbers: number[],
  mode: Mode = { server: false },
): Promise<ApiResult<{ parts: UploadPartURL[] }>> {
  return apiRequest<{ parts: UploadPartURL[] }>(
    `/api/v1/shoots/${shootId}/photos/${photoId}/uploads/parts`,
    { method: "POST", body: JSON.stringify({ partNumbers }) },
    mode,
  );
}

export function completePhotoUpload(
  shootId: string,
  photoId: string,
  parts: CompletePart[],
  mode: Mode = { server: false },
): Promise<ApiResult<Photo>> {
  return apiRequest<Photo>(
    `/api/v1/shoots/${shootId}/photos/${photoId}/complete`,
    { method: "POST", body: JSON.stringify({ parts }) },
    mode,
  );
}

export function listPhotos(
  shootId: string,
  params: { limit?: number; offset?: number } = {},
  mode: Mode = { server: false },
): Promise<ApiResult<PhotoListResult>> {
  const sp = new URLSearchParams();
  if (params.limit != null) sp.set("limit", String(params.limit));
  if (params.offset != null) sp.set("offset", String(params.offset));
  const q = sp.toString();
  return apiRequest<PhotoListResult>(
    `/api/v1/shoots/${shootId}/photos${q ? `?${q}` : ""}`,
    { method: "GET" },
    mode,
  );
}

export function retryPhoto(
  shootId: string,
  photoId: string,
  mode: Mode = { server: false },
): Promise<ApiResult<Photo>> {
  return apiRequest<Photo>(
    `/api/v1/shoots/${shootId}/photos/${photoId}/retry`,
    { method: "POST" },
    mode,
  );
}

export function deletePhoto(
  shootId: string,
  photoId: string,
  mode: Mode = { server: false },
): Promise<ApiResult<void>> {
  return apiRequest<void>(
    `/api/v1/shoots/${shootId}/photos/${photoId}`,
    { method: "DELETE" },
    mode,
  );
}
