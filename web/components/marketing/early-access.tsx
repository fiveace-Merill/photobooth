import Link from "next/link";
import { landing } from "@/lib/content/landing";
import { EarlyAccessForm } from "./early-access-form";

export function EarlyAccess() {
  return (
    <section id={landing.earlyAccess.id} className="dark scroll-mt-20 bg-background text-foreground">
      <div className="mx-auto grid max-w-7xl gap-10 px-4 py-16 sm:px-6 sm:py-20 lg:grid-cols-[minmax(0,1fr)_minmax(0,32rem)] lg:gap-16 lg:px-8">
        <div className="max-w-xl">
          <p className="inline-flex rounded-full bg-volt px-3 py-1 text-sm font-semibold text-volt-foreground">
            Early access
          </p>
          <h2 className="mt-5 text-3xl font-bold tracking-tight sm:text-4xl">{landing.earlyAccess.heading}</h2>
          <p className="mt-5 text-base leading-relaxed text-muted-foreground sm:text-lg">
            {landing.earlyAccess.text}
          </p>
          <Link
            href={landing.earlyAccess.demoLink.href}
            className="mt-8 inline-flex min-h-11 items-center text-sm font-medium text-foreground underline decoration-border underline-offset-4 hover:decoration-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {landing.earlyAccess.demoLink.label}
          </Link>
        </div>
        <div className="rounded-2xl border border-border bg-card p-6 sm:p-8">
          <EarlyAccessForm />
        </div>
      </div>
    </section>
  );
}
