import { describe, expect, it } from "vitest";
import { validateWaitlistInput } from "./waitlist";

const base = {
  name: "Amina Wanjiru",
  contact: "amina@example.com",
  photographyType: "Weddings",
  shootsPerMonth: "1 to 5",
  consent: true,
  website: "",
};

describe("validateWaitlistInput", () => {
  it("accepts a valid email contact", () => {
    const result = validateWaitlistInput(base);
    expect(result.ok).toBe(true);
  });

  it("accepts valid Kenyan phone formats", () => {
    const formats = ["0712345678", "0112345678", "+254712345678", "254712345678"];
    for (const contact of formats) {
      const result = validateWaitlistInput({ ...base, contact });
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.data.normalizedContact).toBe(
          contact.startsWith("01") ? "254112345678" : "254712345678",
        );
        expect(result.data.contactType).toBe("phone");
      }
    }
  });

  it("rejects invalid contact", () => {
    const result = validateWaitlistInput({ ...base, contact: "not-a-contact" });
    expect(result.ok).toBe(false);
    if (!result.ok && !result.silent) {
      expect(result.errors.contact).toBeTruthy();
    }
  });

  it("rejects missing consent", () => {
    const result = validateWaitlistInput({ ...base, consent: false });
    expect(result.ok).toBe(false);
    if (!result.ok && !result.silent) {
      expect(result.errors.consent).toBeTruthy();
    }
  });

  it("rejects honeypot-filled submissions silently", () => {
    const result = validateWaitlistInput({ ...base, website: "https://spam.example" });
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.silent).toBe(true);
    }
  });
});
