"use client";

import { useState } from "react";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { formatMoney } from "@/lib/money";
import { validateKenyanPhone } from "@/lib/phone";
import type { Delivery } from "@/lib/api/types";
import type { PaymentState } from "./payment-machine";

type Props = {
  delivery: Delivery;
  state: PaymentState;
  onClose: () => void;
  onPhoneChange: (phone: string) => void;
  onSubmit: (phone: string) => void;
  onRetry: () => void;
};

export function PaySheet({ delivery, state, onClose, onPhoneChange, onSubmit, onRetry }: Props) {
  const [localError, setLocalError] = useState<string | null>(null);
  const open = state.status !== "idle" && state.status !== "succeeded";

  function submit() {
    if (!validateKenyanPhone(state.phone)) {
      setLocalError("Enter a Safaricom number like 0712 345 678.");
      return;
    }
    setLocalError(null);
    onSubmit(state.phone);
  }

  return (
    <Dialog open={open} onOpenChange={(next) => { if (!next) onClose(); }}>
      <DialogContent
        showCloseButton={false}
        className="top-auto bottom-0 left-0 right-0 max-w-full translate-x-0 translate-y-0 rounded-t-[20px] rounded-b-none border-border bg-card p-6 pb-[calc(1.5rem+env(safe-area-inset-bottom,0px))] text-foreground sm:top-1/2 sm:bottom-auto sm:left-1/2 sm:right-auto sm:max-w-[460px] sm:-translate-x-1/2 sm:-translate-y-1/2 sm:rounded-[20px] sm:pb-6"
      >
        {state.status === "form" || state.status === "idle" ? (
          <div>
            <DialogTitle className="m-0 text-[1.6rem] font-bold leading-[1.1] tracking-[-0.015em] text-foreground">
              Approve and pay
            </DialogTitle>
            <p className="mt-2 mb-4 text-muted-foreground">
              By paying, you confirm you’re happy with these previews. All {delivery.photos.length} photos unlock in original quality.
            </p>
            <dl className="mb-5 border-t border-border">
              <div className="flex justify-between gap-4 border-b border-border py-[0.7rem]">
                <dt className="text-muted-foreground">Total</dt>
                <dd className="m-0 font-semibold">{formatMoney(delivery.priceMinor, delivery.currency)}</dd>
              </div>
              <div className="flex justify-between gap-4 border-b border-border py-[0.7rem]">
                <dt className="text-muted-foreground">Link works until</dt>
                <dd className="m-0 text-right font-semibold">{delivery.expiresAt}</dd>
              </div>
            </dl>
            <label className="mb-2 block font-semibold" htmlFor="phone">M-PESA phone number</label>
            <Input
              id="phone"
              type="tel"
              inputMode="tel"
              autoComplete="tel"
              placeholder="0712 345 678"
              value={state.phone}
              onChange={(e) => { setLocalError(null); onPhoneChange(e.target.value); }}
              className="min-h-[52px] rounded-xl bg-background px-4 text-[1.15rem] tracking-[0.02em]"
            />
            <p className="mt-2 mb-3 min-h-[1.4em] text-sm text-destructive" role="alert">
              {localError ?? state.error}
            </p>
            <div className="flex flex-wrap gap-3">
              <Button type="button" className="min-h-12 flex-1 rounded-full px-6 text-base font-bold" onClick={submit}>
                Pay {formatMoney(delivery.priceMinor, delivery.currency)}
              </Button>
              <Button type="button" variant="outline" className="min-h-12 flex-1 rounded-full px-6 text-base font-bold" onClick={onClose}>
                Not now
              </Button>
            </div>
          </div>
        ) : null}

        {state.status === "waiting" ? (
          <div>
            <div className="mb-4 h-[34px] w-[34px] animate-spin rounded-full border-4 border-border border-t-primary motion-reduce:animate-none" aria-hidden="true" />
            <DialogTitle className="m-0 text-[1.6rem] font-bold leading-[1.1] tracking-[-0.015em]">Check your phone</DialogTitle>
            <p className="mt-2 mb-4 text-muted-foreground">
              Enter your M-PESA PIN to pay {formatMoney(delivery.priceMinor, delivery.currency)} to {delivery.studioName}. Keep this page open.
            </p>
            <div className="flex flex-wrap gap-3">
              <Button type="button" variant="outline" className="min-h-12 rounded-full px-6 text-base font-bold" onClick={onClose}>
                Cancel payment
              </Button>
            </div>
          </div>
        ) : null}

        {state.status === "failed" ? (
          <div>
            <DialogTitle className="m-0 text-[1.6rem] font-bold leading-[1.1] tracking-[-0.015em]">Payment didn’t go through</DialogTitle>
            <p className="mt-2 mb-4 text-muted-foreground">
              You haven’t been charged. Try again, or check that your phone is on and has network.
            </p>
            <div className="flex flex-wrap gap-3">
              <Button type="button" className="min-h-12 flex-1 rounded-full px-6 text-base font-bold" onClick={onRetry}>
                Try again
              </Button>
              <Button type="button" variant="outline" className="min-h-12 flex-1 rounded-full px-6 text-base font-bold" onClick={onClose}>
                Close
              </Button>
            </div>
          </div>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}
