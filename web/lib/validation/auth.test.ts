import { describe, expect, it } from "vitest";
import {
  mapAuthError,
  suggestSlug,
  validatePassword,
  validateSlug,
} from "./auth";

describe("suggestSlug", () => {
  it("creates a lowercase hyphenated slug from business name", () => {
    expect(suggestSlug("  Njeri  Studio & Co ")).toBe("njeri-studio-co");
  });
});

describe("validateSlug", () => {
  it("accepts a valid slug", () => {
    expect(validateSlug("studio-one").ok).toBe(true);
  });

  it("rejects invalid slugs", () => {
    expect(validateSlug("Studio One").ok).toBe(false);
    expect(validateSlug("-bad").ok).toBe(false);
    expect(validateSlug("bad-").ok).toBe(false);
    expect(validateSlug("ab").ok).toBe(false);
  });
});

describe("validatePassword", () => {
  it("accepts passwords with at least 8 characters", () => {
    expect(validatePassword("12345678").ok).toBe(true);
  });

  it("rejects short passwords", () => {
    expect(validatePassword("short").ok).toBe(false);
  });
});

describe("mapAuthError", () => {
  it("maps backend error codes to user messages", () => {
    expect(mapAuthError({ code: "invalid_credentials", status: 401 })).toBe(
      "Email or password is incorrect",
    );
    expect(mapAuthError({ code: "email_taken", status: 409 })).toBe(
      "That email is already in use.",
    );
    expect(mapAuthError({ code: "slug_taken", status: 409 })).toBe(
      "That studio slug is already taken.",
    );
    expect(mapAuthError({ code: "signup_closed", status: 403 })).toBe(
      "signup_closed",
    );
  });

  it("falls back safely for unknown errors", () => {
    expect(mapAuthError({ status: 500 })).toBe("Something went wrong. Please try again.");
  });
});
