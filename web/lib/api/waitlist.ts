import type { ValidWaitlistInput } from "@/lib/validation/waitlist";

export type WaitlistJoinResult =
  | { ok: true }
  | { ok: false; retryable: true; message: string };

export interface WaitlistApi {
  join(input: ValidWaitlistInput): Promise<WaitlistJoinResult>;
}

class RealWaitlistApi implements WaitlistApi {
  async join(input: ValidWaitlistInput): Promise<WaitlistJoinResult> {
    try {
      const res = await fetch("/api/v1/waitlist", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: input.name,
          contact: input.contact,
          photographyType: input.photographyType,
          shootsPerMonth: input.shootsPerMonth,
          consent: input.consent,
          website: "",
        }),
        credentials: "same-origin",
      });

      if (res.ok) return { ok: true };
      if (res.status === 429) {
        return { ok: false, retryable: true, message: "Too many requests. Please wait a moment and try again." };
      }
      return { ok: false, retryable: true, message: "Could not submit right now. Please try again." };
    } catch {
      return { ok: false, retryable: true, message: "Could not submit right now. Please check your connection and try again." };
    }
  }
}

export const waitlistApi: WaitlistApi = new RealWaitlistApi();
