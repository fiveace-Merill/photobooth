"use client";

import { Button } from "@/components/ui/button";
import { formatMoney } from "@/lib/money";
import type { Delivery } from "@/lib/api/types";

type Props = {
  delivery: Delivery;
  onPrimary: () => void;
};

export function ActionBar({ delivery, onPrimary }: Props) {
  const isPreview = delivery.status === "preview";
  return (
    <div className="fixed inset-x-0 bottom-0 z-40 border-t border-border bg-background/92 backdrop-blur-[10px]">
      <div className="mx-auto flex max-w-[1200px] items-center justify-between gap-4 px-4 pb-[calc(0.75rem+env(safe-area-inset-bottom,0px))] pt-3 sm:px-6">
        <div className="min-w-0 leading-tight">
          <strong className="block text-[1.05rem] font-bold">
            {isPreview ? formatMoney(delivery.priceMinor, delivery.currency) : "Your photos are ready"}
          </strong>
          <span className="hidden text-sm text-muted-foreground sm:block">
            {isPreview
              ? `Pay once with M-PESA to unlock all ${delivery.photos.length} photos`
              : `Original quality, about ${Math.round(delivery.totalOriginalBytes / (1024 * 1024))} MB. Wi-Fi recommended.`}
          </span>
        </div>
        <Button
          type="button"
          onClick={onPrimary}
          className="min-h-12 shrink-0 rounded-full px-6 text-base font-bold"
        >
          {isPreview ? "Approve and pay" : "Download all"}
        </Button>
      </div>
    </div>
  );
}
