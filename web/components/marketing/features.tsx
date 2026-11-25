import { landing } from "@/lib/content/landing";
import { FadeIn } from "./fade-in";

export function Features() {
  const { features } = landing;
  return (
    <section className="bg-background" aria-labelledby="features-heading">
      <div className="mx-auto max-w-7xl px-4 py-16 sm:px-6 sm:py-20 lg:px-8">
        <FadeIn>
          <h2 id="features-heading" className="max-w-3xl text-3xl font-bold tracking-tight sm:text-4xl">
            {features.heading}
          </h2>
        </FadeIn>
        <div className="mt-10 grid gap-x-8 gap-y-8 sm:grid-cols-2 xl:grid-cols-3">
          {features.items.map((item) => (
            <FadeIn key={item.title}>
              <article className="border-t border-border pt-5">
                <h3 className="text-lg font-semibold">{item.title}</h3>
                <p className="mt-2 text-sm leading-relaxed text-muted-foreground sm:text-base">{item.body}</p>
              </article>
            </FadeIn>
          ))}
        </div>
      </div>
    </section>
  );
}
