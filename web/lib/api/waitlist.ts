import type { ValidWaitlistInput } from "@/lib/validation/waitlist";

export type WaitlistJoinResult = { ok: true };

export interface WaitlistApi {
  join(input: ValidWaitlistInput): Promise<WaitlistJoinResult>;
}

class MockWaitlistApi implements WaitlistApi {
  async join(input: ValidWaitlistInput): Promise<WaitlistJoinResult> {
    await new Promise((resolve) => setTimeout(resolve, 600));
    console.info("[waitlist mock] early access request", input);
    return { ok: true };
  }
}

// Swapping mock for a real Go API call later should be a one-file change.
export const waitlistApi: WaitlistApi = new MockWaitlistApi();
