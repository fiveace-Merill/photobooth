export type ValidationResult = { ok: true } | { ok: false; message: string };

const SLUG_RE = /^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$/;

export function suggestSlug(name: string): string {
  return name
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9\s-]/g, "")
    .replace(/\s+/g, "-")
    .replace(/-+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 32)
    .replace(/-+$/g, "");
}

export function validateSlug(slug: string): ValidationResult {
  if (!SLUG_RE.test(slug)) {
    return {
      ok: false,
      message: "Use 3-32 characters: lowercase letters, numbers, and hyphens; no leading or trailing hyphen.",
    };
  }
  return { ok: true };
}

export function validatePassword(password: string): ValidationResult {
  if (password.length < 8) {
    return { ok: false, message: "Password must be at least 8 characters." };
  }
  return { ok: true };
}

export function validateEmail(email: string): ValidationResult {
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim())) {
    return { ok: false, message: "Enter a valid email address." };
  }
  return { ok: true };
}

export type AuthErrorLike = { code?: string; status?: number; error?: string };

export function mapAuthError(err: AuthErrorLike): string {
  switch (err.code) {
    case "invalid_credentials":
      return "Email or password is incorrect";
    case "email_taken":
      return "That email is already in use.";
    case "slug_taken":
      return "That studio slug is already taken.";
    case "signup_closed":
      return "signup_closed";
    case "invalid_input":
      return err.error || "Check the form and try again.";
    case "unauthorized":
      return "Your session has expired. Please log in again.";
    default:
      if (err.status === 401) return "Email or password is incorrect";
      return "Something went wrong. Please try again.";
  }
}
