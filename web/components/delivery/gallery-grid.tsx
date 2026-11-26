"use client";

import type { Photo } from "@/lib/api/types";
import { PhotoTile } from "./photo-tile";

type Props = {
  photos: Photo[];
  unlocked: boolean;
  onOpen: (index: number) => void;
};

export function GalleryGrid({ photos, unlocked, onOpen }: Props) {
  return (
    <ul
      className="m-0 list-none columns-2 gap-2 p-0 md:columns-3 md:gap-3 xl:columns-4"
      aria-label="Photos"
    >
      {photos.map((photo, i) => (
        <PhotoTile
          key={photo.id}
          photo={photo}
          index={i}
          total={photos.length}
          unlocked={unlocked}
          onOpen={onOpen}
          priority={i === 0}
        />
      ))}
    </ul>
  );
}
