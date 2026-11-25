import type { Delivery } from "@/lib/api/types";

export function StatusBanner({ delivery }: { delivery: Delivery }) {
  if (delivery.status === "preview") {
    return (
      <div className="mt-6 max-w-[56ch]" id="status">
        <p className="m-0 text-base leading-relaxed text-foreground">
          These are previews with a watermark. Approve and pay to unlock every photo in original quality.
        </p>
      </div>
    );
  }

  return (
    <div className="mt-6 max-w-[56ch]" id="status">
      <span className="mb-3 inline-block rounded-full bg-volt px-3 py-1 text-sm font-bold text-volt-foreground">
        Unlocked
      </span>
      <p className="m-0 text-base leading-relaxed text-foreground">
        Your photos are ready in original quality. This link works until {delivery.expiresAt}, so download them before then.
      </p>
    </div>
  );
}
