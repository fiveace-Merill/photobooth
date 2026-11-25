"use client";

import type { CSSProperties } from "react";
import type { Photo } from "@/lib/api/types";

type Props = {
  photo: Photo;
  index: number;
  total: number;
  unlocked: boolean;
  onOpen: (index: number) => void;
};

const TONES: [string, string, string][] = [
  ["#2d3126", "#111112", "rgba(244,243,241,.28)"],
  ["#3d4332", "#1a1b17", "rgba(210,255,0,.18)"],
  ["#7f8177", "#2d3126", "rgba(244,243,241,.35)"],
  ["#22241e", "#5b5e52", "rgba(244,243,241,.22)"],
  ["#a3a59b", "#3a3f31", "rgba(17,17,18,.25)"],
];

export function photoPlaceholderBackground(index: number): string {
  const t = TONES[index % TONES.length];
  const x = 20 + ((index * 17) % 60);
  const y = 18 + ((index * 11) % 40);
  const deg = 150 + ((index * 23) % 60);
  return `radial-gradient(70% 55% at ${x}% ${y}%, ${t[2]}, transparent 70%), linear-gradient(${deg}deg, ${t[0]}, ${t[1]})`;
}

export function PhotoTile({ photo, index, total, unlocked, onOpen }: Props) {
  const ratio = photo.width / photo.height;
  const style = {
    "--r": String(ratio),
    background: photo.previewUrl ? undefined : photoPlaceholderBackground(index),
  } as CSSProperties;

  return (
    <li className="mb-2 break-inside-avoid md:mb-3">
      <button
        type="button"
        className="block w-full cursor-pointer overflow-hidden rounded-[10px] border-0 bg-transparent p-0 focus-visible:outline-ring"
        aria-label={`Open photo ${index + 1} of ${total}`}
        onClick={() => onOpen(index)}
      >
        <div
          className="relative w-full overflow-hidden rounded-[10px] [aspect-ratio:var(--r)]"
          style={style}
        >
          {photo.previewUrl ? (
            // Keep next/image swap in this one place when real preview URLs exist.
            // <Image src={photo.previewUrl} alt="" width={photo.width} height={photo.height}
            //   sizes="(min-width:1100px) 25vw, (min-width:720px) 33vw, 50vw" className="h-auto w-full" />
            // eslint-disable-next-line @next/next/no-img-element
            <img
              src={photo.previewUrl}
              alt=""
              width={photo.width}
              height={photo.height}
              loading="lazy"
              className="h-full w-full object-cover"
            />
          ) : null}
          <div
            aria-hidden="true"
            className="pointer-events-none absolute inset-0 transition-opacity duration-[800ms] ease-out data-[gone=true]:opacity-0 motion-reduce:transition-none"
            data-gone={unlocked ? "true" : "false"}
            style={{
              transitionDelay: unlocked ? `${index * 70}ms` : "0ms",
              backgroundImage:
                "url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='170' height='120'%3E%3Ctext x='85' y='66' text-anchor='middle' transform='rotate(-24 85 60)' font-family='sans-serif' font-size='17' font-weight='700' fill='white' fill-opacity='.4'%3EPREVIEW%3C/text%3E%3C/svg%3E\")",
            }}
          />
        </div>
      </button>
    </li>
  );
}
