import { formatMoney } from "@/lib/money";
import { PhotoFrame } from "@/components/delivery/photo-tile";

const photos = [
  { id: "hero-1", width: 800, height: 1200, thumbUrl: "/demo/previews/aleksandra-sapozhnikova.jpg", previewUrl: "/demo/previews/aleksandra-sapozhnikova.jpg" },
  { id: "hero-2", width: 1200, height: 801, thumbUrl: "/demo/previews/bryan-heng.jpg", previewUrl: "/demo/previews/bryan-heng.jpg" },
  { id: "hero-3", width: 750, height: 1200, thumbUrl: "/demo/previews/eren-yildiz.jpg", previewUrl: "/demo/previews/eren-yildiz.jpg" },
  { id: "hero-4", width: 1200, height: 800, thumbUrl: "/demo/previews/modern-dairy-portraits.jpg", previewUrl: "/demo/previews/modern-dairy-portraits.jpg" },
];

export function HeroPreview() {
  return (
    <div
      aria-hidden="true"
      className="pointer-events-none mx-auto w-full max-w-[22rem] sm:max-w-[24rem] lg:max-w-[26rem]"
    >
      <div className="rounded-[2rem] border border-border/60 bg-card p-3 shadow-2xl shadow-black/30">
        <div className="rounded-[1.5rem] bg-background p-4 text-foreground">
          <p className="text-xs font-medium text-muted-foreground">Nairobi Studio</p>
          <p className="mt-2 text-2xl font-bold leading-tight tracking-tight">Amina &amp; David</p>
          <p className="mt-1 text-xs text-muted-foreground">24 photos</p>
          <div className="mt-3 inline-flex rounded-full bg-volt px-2.5 py-1 text-[11px] font-bold text-volt-foreground">
            Photos unlocked
          </div>
          <div className="mt-4 grid grid-cols-2 gap-2">
            {photos.map((photo, index) => (
              <PhotoFrame
                key={photo.id}
                photo={photo}
                index={index}
                total={photos.length}
                unlocked={true}
                priority={index === 0}
                interactive={false}
                sizes="(min-width: 1024px) 13vw, (min-width: 640px) 20vw, 40vw"
                className="relative w-full overflow-hidden rounded-lg [aspect-ratio:var(--r)]"
              />
            ))}
          </div>
          <div className="mt-4 rounded-xl border border-border bg-card p-3">
            <p className="text-[11px] text-muted-foreground">Paid with M-PESA</p>
            <p className="text-sm font-semibold">{formatMoney(300000, "KES")}</p>
            <div className="mt-2 inline-flex min-h-9 items-center justify-center rounded-lg bg-primary px-3 text-xs font-medium text-primary-foreground">
              Download all originals
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
