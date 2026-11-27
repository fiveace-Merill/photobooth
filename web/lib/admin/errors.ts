import { messageForApiError, type ApiError } from "@/lib/api/errors";

const adminMessages: Record<string, string> = {
  invalid_credentials: "Invalid email, password, or code.",
  unauthorized: "Your admin session has expired. Please log in again.",
  rate_limited: "Too many requests. Please wait a moment and try again.",
  not_found: "This item could not be found.",
  invalid_input: "Check the fields and try again.",
  invalid_json: "The request was invalid. Please try again.",
  invalid_origin: "The request could not be sent. Please try again.",
  invalid_content_type: "The request could not be sent. Please try again.",
  internal: "Something went wrong. Please try again.",
};

export function messageForAdminError(err: Pick<ApiError, "code" | "status" | "error">): string {
  if (err.code && adminMessages[err.code]) return adminMessages[err.code];
  return messageForApiError(err);
}
