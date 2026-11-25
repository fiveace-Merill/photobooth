import { landing } from "@/lib/content/landing";
import { FadeIn } from "./fade-in";

export function Problems() {
  const { problems } = landing;
  return (
    <section className="bg-background" aria-labelledby="problems-heading">
      <div className="mx-auto max-w-7xl px-4 py-16 sm:px-6 sm:py-20 lg:px-8">
        <FadeIn>
          <h2 id="problems-heading" className="max-w-2xl text-3xl font-bold tracking-tight sm:text-4xl">
            {problems.heading}
          </h2>
        </FadeIn>
        <div className="mt-10 grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
          {problems.items.map((item) => (
            <FadeIn key={item.title}>
              <article className="h-full rounded-2xl border border-border bg-card p-6">
                <h3 className="text-lg font-semibold leading-snug">{item.title}</h3>
                <p className="mt-3 text-sm leading-relaxed text-muted-foreground sm:text-base">{item.body}</p>
              </article>
            </FadeIn>
          ))}
        </div>
      </div>
    </section>
  );
}
