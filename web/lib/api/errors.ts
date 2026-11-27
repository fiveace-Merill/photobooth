export type ApiError = {
  ok: false;
  status: number;
  code?: string;
  error: string;
  existingClientId?: string;
};

export type ApiOk<T> = { ok: true; data: T };
export type ApiResult<T> = ApiOk<T> | ApiError;

const messages: Record<string, string> = {
  invalid_json: "The request was invalid. Please try again.",
  invalid_input: "Check the highlighted fields and try again.",
  unauthorized: "Your session has expired. Please log in again.",
  not_found: "This item could not be found.",
  client_not_found: "The selected client could not be found.",
  client_phone_exists: "That phone number is already used by another client.",
  client_has_shoots: "This client has shoots and cannot be deleted.",
  shoot_not_deletable: "This shoot can no longer be deleted.",
  invalid_status_transition: "That status change is not allowed.",
  invalid_content_type: "The request could not be sent. Please try again.",
  invalid_origin: "The request could not be sent. Please try again.",
  signup_closed: "Signups are invite-only right now.",
  invite_invalid: "This invitation isn't valid or has expired.",
  unsupported_type: "This file type isn't supported. Use JPEG, PNG, or WebP.",
  file_too_large: "This file is too large to upload.",
  quota_exceeded: "This upload would exceed your storage quota.",
  too_many_photos: "This shoot has reached its photo limit.",
  shoot_not_uploadable: "This shoot can't accept uploads right now.",
  invalid_photo_state: "This photo can't be changed in its current state.",
  size_mismatch: "The uploaded file didn't match the expected size. Please try again.",
  already_delivered: "This shoot already has an active delivery link.",
  shoot_not_deliverable: "This shoot is not ready for delivery yet.",
  already_unlocked: "Originals are already unlocked.",
  invalid_mark_paid: "This shoot cannot be marked as paid.",
  no_active_link: "There is no active delivery link.",
  not_delivered: "This shoot has not been delivered yet.",
  originals_locked: "Originals are locked until payment is confirmed.",
  rate_limited: "Too many requests. Please wait a moment and try again.",
  channel_unavailable: "Client email is unavailable. Add or fix their email address.",
  too_many_sends: "Too many delivery emails were sent for this shoot recently. Try again later.",
  email_daily_limit_reached: "You've reached today's email limit. Try again tomorrow.",
  email_unavailable: "Email delivery is not configured right now.",
  internal: "Something went wrong. Please try again.",
};

export function messageForApiCode(code?: string): string {
  if (!code) return "Something went wrong. Please try again.";
  return messages[code] ?? "Something went wrong. Please try again.";
}

export function messageForApiError(err: Pick<ApiError, "code" | "status" | "error">): string {
  if (err.code) return messageForApiCode(err.code);
  if (err.status === 401) return messages.unauthorized;
  return err.error || "Something went wrong. Please try again.";
}

export async function toApiError(res: Response): Promise<ApiError> {
  const body = (await res.json().catch(() => ({}))) as {
    error?: string;
    code?: string;
    existingClientId?: string;
  };
  return {
    ok: false,
    status: res.status,
    code: body.code,
    error: body.error || "Request failed",
    existingClientId: body.existingClientId,
  };
}
