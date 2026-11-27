"use client";

import { Button } from "@/components/ui/button";
import { formatMoney } from "@/lib/money";
import { formatKenyanPhone } from "@/lib/phone";
import type { Delivery } from "@/lib/api/types";

type Props = {
  delivery: Delivery;
  onPrimary: () => void;
};

function whatsappLink(phone: string, delivery: Delivery): string {
  const text = `Hi ${delivery.studioName}, I'd like to pay for ${delivery.title}.`;
  return `https://wa.me/${phone}?text=${encodeURIComponent(text)}`;
}

export function ActionBar({ delivery, onPrimary }: Props) {
  const isPreview = delivery.status === "preview";
  const manual = isPreview && delivery.features.payments === "manual";
  const phone = delivery.studioPhone;

  return (
    <div className="fixed inset-x-0 bottom-0 z-40 border-t border-border bg-background/92 backdrop-blur-[10px]">
      <div className="mx-auto flex max-w-[1200px] flex-col gap-3 px-4 pb-[calc(0.75rem+env(safe-area-inset-bottom,0px))] pt-3 sm:flex-row sm:items-center sm:justify-between sm:px-6">
        <div className="min-w-0 leading-tight">
          <strong className="block text-[1.05rem] font-bold">
            {isPreview ? formatMoney(delivery.priceMinor, delivery.currency) : "Your photos are ready"}
          </strong>
          <span className="text-sm text-muted-foreground">
            {isPreview
              ? manual
                ? `Contact ${delivery.studioName} to pay and unlock all ${delivery.photos.length} photos.`
                : `Pay once with M-PESA to unlock all ${delivery.photos.length} photos`
              : `Original quality, about ${Math.round(delivery.totalOriginalBytes / (1024 * 1024))} MB. Wi-Fi recommended.`}
          </span>
        </div>
        <div className="flex shrink-0 flex-wrap gap-2">
          {manual && phone ? (
            <>
              <Button variant="outline" className="min-h-12 rounded-full px-6 text-base font-bold" onClick={() => { window.location.href = `tel:+${phone}`; }}>
                Call {formatKenyanPhone(phone)}
              </Button>
              <Button className="min-h-12 rounded-full px-6 text-base font-bold" onClick={() => { window.open(whatsappLink(phone, delivery), "_blank", "noopener,noreferrer"); }}>
                WhatsApp {delivery.studioName}
              </Button>
            </>
          ) : manual ? (
            <Button type="button" className="min-h-12 rounded-full px-6 text-base font-bold" onClick={onPrimary}>
              Contact {delivery.studioName}
            </Button>
          ) : isPreview ? (
            <Button type="button" onClick={onPrimary} className="min-h-12 rounded-full px-6 text-base font-bold">
              Approve and pay
            </Button>
          ) : (
            <Button className="min-h-12 rounded-full px-6 text-base font-bold" onClick={() => { window.location.href = delivery.downloadAllUrl; }}>
              Download all
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}
