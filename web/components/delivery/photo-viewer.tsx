"use client";

import Image from "next/image";
import { useEffect } from "react";
import type { CSSProperties } from "react";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import type { Photo } from "@/lib/api/types";
import { photoPlaceholderBackground } from "./photo-tile";

type Props = {
  photos: Photo[];
  index: number | null;
  unlocked: boolean;
  onClose: () => void;
  onPrev: () => void;
  onNext: () => void;
};

export function PhotoViewer({ photos, index, unlocked, onClose, onPrev, onNext }: Props) {
  const open = index !== null;
  const current = index !== null ? photos[index] : null;

  useEffect(() => {
    if (!open) return;
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === "ArrowLeft") {
        e.preventDefault();
        onPrev();
      }
      if (e.key === "ArrowRight") {
        e.preventDefault();
        onNext();
      }
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [open, onPrev, onNext]);

  const ratio = current ? current.width / current.height : 1;
  const style = {
    "--r": String(ratio),
    background: current && !current.previewUrl ? photoPlaceholderBackground(index ?? 0) : undefined,
  } as CSSProperties;

  return (
    <Dialog open={open} onOpenChange={(next) => { if (!next) onClose(); }}>
      <DialogContent
        showCloseButton={false}
        className="w-[min(96vw,920px)] max-w-[96vw] gap-0 rounded-[14px] border-border bg-background p-0 text-foreground sm:max-w-[920px]"
        aria-label="Photo viewer"
      >
        <DialogTitle className="sr-only">Photo viewer</DialogTitle>
        <div className="grid place-items-center p-4">
          <div
            className="relative w-[min(100%,calc(66dvh*var(--r)))] overflow-hidden rounded-[10px] [aspect-ratio:var(--r)]"
            style={style}
          >
            {current?.previewUrl ? (
              <Image
                src={current.previewUrl}
                alt=""
                width={current.width}
                height={current.height}
                sizes="(min-width: 920px) 920px, 96vw"
                className="h-full w-full object-cover"
              />
            ) : null}
            <div
              aria-hidden="true"
              className="pointer-events-none absolute inset-0 transition-opacity duration-[800ms] ease-out data-[gone=true]:opacity-0 motion-reduce:transition-none"
              data-gone={unlocked ? "true" : "false"}
              style={{
                // In production the watermark is baked into the preview file on the server.
                // This overlay is only a visual stand-in for the prototype/mock.
                backgroundImage:
                  "url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='170' height='120'%3E%3Ctext x='85' y='66' text-anchor='middle' transform='rotate(-24 85 60)' font-family='sans-serif' font-size='17' font-weight='700' fill='white' fill-opacity='.4'%3EPREVIEW%3C/text%3E%3C/svg%3E\")",
              }}
            />
          </div>
        </div>
        <div className="flex flex-wrap items-center justify-between gap-2 px-4 pb-4 pt-2">
          <span className="text-sm text-foreground">
            {index !== null ? `Photo ${index + 1} of ${photos.length}` : ""}
          </span>
          <span className="text-sm text-muted-foreground">
            {unlocked ? "Original quality" : "Preview with watermark"}
          </span>
          <div className="flex flex-wrap gap-2">
            <Button type="button" variant="outline" className="min-h-11 rounded-full px-4" onClick={onPrev} aria-label="Previous photo">
              Previous
            </Button>
            <Button type="button" variant="outline" className="min-h-11 rounded-full px-4" onClick={onNext} aria-label="Next photo">
              Next
            </Button>
            <Button type="button" variant="outline" className="min-h-11 rounded-full px-4" onClick={onClose}>
              Close
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
