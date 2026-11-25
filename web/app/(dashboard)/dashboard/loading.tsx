export default function DashboardLoading() {
  return (
    <main className="mx-auto max-w-4xl px-6 py-16">
      <div className="h-4 w-24 animate-pulse rounded bg-muted" />
      <div className="mt-4 h-10 w-72 animate-pulse rounded bg-muted" />
      <div className="mt-4 h-4 w-56 animate-pulse rounded bg-muted" />
      <div className="mt-10 h-32 animate-pulse rounded-xl border border-border bg-card" />
    </main>
  );
}
