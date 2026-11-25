import Link from "next/link";
import { landing } from "@/lib/content/landing";
import { FadeIn } from "./fade-in";

export function HowItWorks() {
  const { howItWorks } = landing;
  return (
    <section id="how-it-works" className="scroll-mt-20 bg-muted/30" aria-labelledby="how-heading">
      <div className="mx-auto max-w-7xl px-4 py-16 sm:px-6 sm:py-20 lg:px-8">
        <FadeIn>
          <h2 id="how-heading" className="max-w-2xl text-3xl font-bold tracking-tight sm:text-4xl">
            {howItWorks.heading}
          </h2>
        </FadeIn>
        <ol className="mt-10 grid gap-6 md:grid-cols-2 xl:grid-cols-4">
          {howItWorks.steps.map((step, i) => (
            <li key={step.title}>
              <FadeIn className="h-full">
                <article className="h-full rounded-2xl border border-border bg-card p-6">
                  <span className="inline-flex h-9 w-9 items-center justify-center rounded-full bg-volt text-sm font-bold text-volt-foreground">
                    {i + 1}
                  </span>
                  <h3 className="mt-4 text-lg font-semibold">{step.title}</h3>
                  <p className="mt-3 text-sm leading-relaxed text-muted-foreground sm:text-base">{step.body}</p>
                </article>
              </FadeIn>
            </li>
          ))}
        </ol>
        <FadeIn className="mt-8">
          <Link
            href={howItWorks.demoLink.href}
            className="inline-flex min-h-11 items-center text-sm font-medium text-foreground underline decoration-border underline-offset-4 hover:decoration-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {howItWorks.demoLink.label}
          </Link>
        </FadeIn>
      </div>
    </section>
  );
}
