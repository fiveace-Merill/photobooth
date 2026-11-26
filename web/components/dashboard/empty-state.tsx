export function EmptyState({ title, description }: { title: string; description: string }) {
  return (
    <section className="rounded-xl border border-border bg-card p-6">
      <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
      <p className="mt-2 text-sm text-muted-foreground">{description}</p>
    </section>
  );
}
