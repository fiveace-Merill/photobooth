import { landing } from "@/lib/content/landing";
import { testimonials } from "@/lib/content/testimonials";
import { FadeIn } from "./fade-in";

export function Testimonials() {
  return (
    <section className="bg-background" aria-labelledby="testimonials-heading">
      <div className="mx-auto max-w-7xl px-4 py-16 sm:px-6 sm:py-20 lg:px-8">
        <FadeIn>
          <div className="flex flex-wrap items-end gap-3">
            <h2 id="testimonials-heading" className="text-3xl font-bold tracking-tight sm:text-4xl">
              {landing.testimonials.heading}
            </h2>
            <span className="rounded-full border border-border px-2.5 py-1 text-xs font-medium text-muted-foreground">
              Sample
            </span>
          </div>
        </FadeIn>
        <div className="mt-10 grid gap-6 md:grid-cols-3">
          {testimonials.map((item) => (
            <FadeIn key={item.name} className="h-full">
              <figure className="flex h-full flex-col rounded-2xl border border-border bg-card p-6">
                <blockquote className="text-base leading-relaxed text-foreground">
                  “{item.quote}”
                </blockquote>
                <figcaption className="mt-6 border-t border-border pt-4 text-sm text-muted-foreground">
                  <span className="block font-medium text-foreground">{item.name}</span>
                  <span>
                    {item.role}, {item.location}
                  </span>
                </figcaption>
              </figure>
            </FadeIn>
          ))}
        </div>
      </div>
    </section>
  );
}
