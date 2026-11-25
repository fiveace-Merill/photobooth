import Link from "next/link";
import { landing } from "@/lib/content/landing";
import { HeroPreview } from "./hero-preview";

export function Hero() {
  const { hero } = landing;
  return (
    <section className="dark bg-background text-foreground">
      <div className="mx-auto grid max-w-7xl gap-12 px-4 py-16 sm:px-6 md:py-20 lg:grid-cols-[minmax(0,1fr)_minmax(0,28rem)] lg:items-center lg:gap-16 lg:px-8 lg:py-24">
        <div className="max-w-2xl">
          <p className="inline-flex rounded-full bg-volt px-3 py-1 text-sm font-semibold text-volt-foreground">
            For photographers in Kenya
          </p>
          <h1 className="mt-6 max-w-[12ch] text-4xl font-bold tracking-tight text-balance sm:text-5xl lg:text-6xl">
            {hero.h1}
          </h1>
          <p className="mt-6 max-w-xl text-base leading-relaxed text-muted-foreground sm:text-lg">
            {hero.sub}
          </p>
          <div className="mt-8 flex flex-col gap-3 sm:flex-row sm:items-center">
            <Link
              href={hero.primaryCta.href}
              className="inline-flex min-h-11 items-center justify-center rounded-lg bg-primary px-5 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              {hero.primaryCta.label}
            </Link>
            <Link
              href={hero.secondaryCta.href}
              className="inline-flex min-h-11 items-center justify-center rounded-lg border border-border px-5 text-sm font-medium text-foreground transition-colors hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              {hero.secondaryCta.label}
            </Link>
          </div>
          <p className="mt-5 text-sm text-muted-foreground">{hero.note}</p>
        </div>
        <HeroPreview />
      </div>
    </section>
  );
}
