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
