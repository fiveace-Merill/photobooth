"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";

type Props = {
  onSubmit: (rating: number, comment: string) => Promise<void>;
};

export function ReviewCard({ onSubmit }: Props) {
  const [rating, setRating] = useState(0);
  const [comment, setComment] = useState("");
  const [sent, setSent] = useState(false);

  if (sent) {
    return (
      <section className="mt-12 mb-4 max-w-[560px] rounded-[14px] border border-border bg-card p-6" aria-labelledby="rv-h">
        <h2 id="rv-h" className="m-0 text-2xl font-semibold tracking-[-0.01em]">Thank you</h2>
        <p className="mt-2 m-0 text-muted-foreground">Your review was sent to Wanjiru.</p>
      </section>
    );
  }

  return (
    <section className="mt-12 mb-4 max-w-[560px] rounded-[14px] border border-border bg-card p-6" aria-labelledby="rv-h">
      <div>
        <h2 id="rv-h" className="m-0 text-2xl font-semibold tracking-[-0.01em]">How was your experience?</h2>
        <p className="mt-1 mb-4 text-muted-foreground">Your review helps Wanjiru reach more clients.</p>
        <div className="mb-4 flex gap-1" role="group" aria-label="Rating">
          {[1, 2, 3, 4, 5].map((n) => (
            <button
              key={n}
              type="button"
              className="grid h-11 w-11 cursor-pointer place-items-center border-0 bg-transparent text-border data-[pressed=true]:text-volt"
              aria-label={`${n} ${n === 1 ? "star" : "stars"}`}
              aria-pressed={rating >= n ? "true" : "false"}
              data-pressed={rating >= n ? "true" : "false"}
              onClick={() => setRating(n)}
            >
              <svg viewBox="0 0 24 24" aria-hidden="true" className="h-8 w-8 fill-current">
                <path d="M12 2.5l2.9 6.1 6.6.8-4.9 4.6 1.3 6.6L12 17.3 6.1 20.6l1.3-6.6L2.5 9.4l6.6-.8z" />
              </svg>
            </button>
          ))}
        </div>
        <Textarea
          aria-label="Your review"
          placeholder="Tell Wanjiru what you liked (optional)"
          value={comment}
          onChange={(e) => setComment(e.target.value)}
          className="mb-4 min-h-[84px] rounded-[10px] bg-background"
        />
        <Button
          type="button"
          className="min-h-12 rounded-full px-6 text-base font-bold"
          disabled={rating === 0}
          onClick={async () => {
            await onSubmit(rating, comment);
            setSent(true);
          }}
        >
          Send review
        </Button>
      </div>
    </section>
  );
}
