"use client";

import { useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

type Props = {
  disabled?: boolean;
  onFiles: (files: File[]) => void;
};

export function PhotoUploader({ disabled, onFiles }: Props) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [dragging, setDragging] = useState(false);

  function pick() { if (!disabled) inputRef.current?.click(); }
  function handleFiles(list: FileList | null) {
    if (!list || list.length === 0) return;
    onFiles(Array.from(list));
    if (inputRef.current) inputRef.current.value = "";
  }

  return (
    <div className="space-y-2">
      <div
        role="button"
        tabIndex={disabled ? -1 : 0}
        aria-disabled={disabled || undefined}
        aria-label="Add photos"
        className={cn(
          "rounded-lg border border-dashed p-6 text-center outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring",
          dragging ? "border-primary bg-muted/50" : "border-border bg-background",
          disabled ? "opacity-60" : "cursor-pointer hover:bg-muted/30",
        )}
        onClick={pick}
        onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); pick(); } }}
        onDragOver={(e) => { e.preventDefault(); if (!disabled) setDragging(true); }}
        onDragLeave={() => setDragging(false)}
        onDrop={(e) => { e.preventDefault(); setDragging(false); if (disabled) return; handleFiles(e.dataTransfer.files); }}
      >
        <p className="text-sm font-medium">Drop photos here, or browse</p>
        <p className="mt-1 text-xs text-muted-foreground">JPEG, PNG, or WebP. You can select multiple files.</p>
        <div className="mt-4 flex justify-center">
          <Button type="button" variant="outline" className="min-h-11" disabled={disabled} onClick={(e) => { e.stopPropagation(); pick(); }}>Choose photos</Button>
        </div>
        <input
          ref={inputRef}
          type="file"
          className="hidden"
          accept="image/jpeg,image/png,image/webp"
          multiple
          onChange={(e) => handleFiles(e.target.files)}
        />
      </div>
      <p className="text-xs text-muted-foreground">Closing this tab interrupts uploads.</p>
    </div>
  );
}
