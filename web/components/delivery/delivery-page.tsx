"use client";

import { useEffect, useReducer, useRef, useState } from "react";
import Link from "next/link";
import { toast } from "sonner";
import type { Delivery } from "@/lib/api/types";
import { deliveryApi } from "@/lib/api";
import { normalizeKenyanPhone } from "@/lib/phone";
import { StatusBanner } from "./status-banner";
import { GalleryGrid } from "./gallery-grid";
import { PhotoViewer } from "./photo-viewer";
import { PaySheet } from "./pay-sheet";
import { ActionBar } from "./action-bar";
import { ReviewCard } from "./review-card";
import { initialPaymentState, paymentReducer } from "./payment-machine";

type Props = {
  delivery: Delivery;
  token: string;
};

export function DeliveryPage({ delivery: initialDelivery, token }: Props) {
  const [delivery, setDelivery] = useState<Delivery>(initialDelivery);
  const [viewerIndex, setViewerIndex] = useState<number | null>(null);
  const [payment, dispatch] = useReducer(paymentReducer, initialPaymentState);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const unlocked = delivery.status === "unlocked";

  useEffect(() => {
    if (payment.status !== "waiting" || !payment.paymentId) return;

    pollRef.current = setInterval(async () => {
      try {
        const result = await deliveryApi.getPaymentStatus(token, payment.paymentId!);
        if (result.status === "succeeded") {
          if (pollRef.current) clearInterval(pollRef.current);
          dispatch({ type: "SUCCEEDED" });
          const fresh = await deliveryApi.getDelivery(token === "demo" ? "demo-paid" : token);
          if (fresh && fresh.status === "unlocked") {
            setDelivery(fresh);
          } else if (delivery.status === "preview") {
            // Mock fallback: unlock locally only if server-shaped unlocked data unavailable.
            setDelivery({
              ...delivery,
              status: "unlocked",
              downloadAllUrl: `/mock/downloads/${token}.zip`,
              totalOriginalBytes: 96 * 1024 * 1024,
              photos: delivery.photos.map((p) => ({ ...p, originalUrl: `/mock/originals/${p.id}.jpg` })),
            });
          }
          const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
          window.scrollTo({ top: 0, behavior: reduceMotion ? "auto" : "smooth" });
          toast("Payment received. Your photos are unlocked.");
        }
        if (result.status === "failed") {
          if (pollRef.current) clearInterval(pollRef.current);
          dispatch({ type: "FAILED", error: "Payment didn’t go through" });
        }
      } catch {
        if (pollRef.current) clearInterval(pollRef.current);
        dispatch({ type: "FAILED", error: "Payment didn’t go through" });
      }
    }, 1200);

    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, [payment.status, payment.paymentId, token, delivery]);

  function openPayment() {
    dispatch({ type: "OPEN" });
  }

  async function submitPayment(phone: string) {
    const normalized = normalizeKenyanPhone(phone);
    const result = await deliveryApi.startPayment(token, normalized);
    dispatch({ type: "SUBMIT", phone, paymentId: result.paymentId });
  }

  return (
    <div className="pb-28 [padding-top:env(safe-area-inset-top,0px)]">
      <div className="mx-auto max-w-[1200px] px-4 sm:px-6">
        <header className="pb-6 pt-8">
          <p className="m-0 mb-3 font-medium text-muted-foreground">{delivery.studioName}</p>
          <h1 className="m-0 max-w-[14ch] text-[clamp(2.25rem,8vw,4.75rem)] font-bold leading-none tracking-[-0.025em]">
            {delivery.title}
          </h1>
          <p className="mt-4 mb-0 text-muted-foreground">
            Shot on {delivery.shotOn}. {delivery.photos.length} photos.
          </p>
          <StatusBanner delivery={delivery} />
        </header>

        <main>
          <GalleryGrid
            photos={delivery.photos}
            unlocked={unlocked}
            onOpen={(i) => setViewerIndex(i)}
          />

          {unlocked ? (
            <ReviewCard
              onSubmit={async (rating, comment) => {
                await deliveryApi.submitReview(token, rating, comment);
              }}
            />
          ) : null}

          <details className="my-8 max-w-[560px] rounded-[10px] border border-dashed border-border p-3 text-sm text-muted-foreground">
            <summary className="cursor-pointer">Prototype controls</summary>
            <p className="mt-2 mb-0">
              Use a phone number ending in 000 to test the payment failure path. Any other valid Safaricom number succeeds after ~3 seconds.
            </p>
          </details>
        </main>

        <footer className="py-8 text-sm text-muted-foreground">
          <Link href="/" className="rounded-sm underline-offset-4 hover:text-foreground hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
            Delivered with Photobooth
          </Link>
        </footer>
      </div>

      <ActionBar
        delivery={delivery}
        onPrimary={() => {
          if (delivery.status === "preview") openPayment();
          else toast("Preparing your zip file…");
        }}
      />

      <PhotoViewer
        photos={delivery.photos}
        index={viewerIndex}
        unlocked={unlocked}
        onClose={() => setViewerIndex(null)}
        onPrev={() =>
          setViewerIndex((i) =>
            i === null ? null : (i + delivery.photos.length - 1) % delivery.photos.length,
          )
        }
        onNext={() =>
          setViewerIndex((i) => (i === null ? null : (i + 1) % delivery.photos.length))
        }
      />

      <PaySheet
        delivery={delivery}
        state={payment}
        onClose={() => {
          if (pollRef.current) clearInterval(pollRef.current);
          dispatch({ type: "CLOSE" });
        }}
        onPhoneChange={(phone) => dispatch({ type: "SET_PHONE", phone })}
        onSubmit={submitPayment}
        onRetry={() => dispatch({ type: "RETRY" })}
      />
    </div>
  );
}
