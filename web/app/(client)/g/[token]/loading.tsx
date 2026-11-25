export default function LoadingDelivery() {
  return (
    <div className="mx-auto max-w-[1200px] px-4 pb-28 pt-8 sm:px-6">
      <div className="h-4 w-48 animate-pulse rounded bg-muted" />
      <div className="mt-4 h-16 w-full max-w-xl animate-pulse rounded bg-muted" />
      <div className="mt-4 h-4 w-64 animate-pulse rounded bg-muted" />
      <div className="mt-8 grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4">
        {Array.from({ length: 8 }).map((_, i) => (
          <div key={i} className="aspect-[4/5] animate-pulse rounded-[10px] bg-muted" />
        ))}
      </div>
    </div>
  );
}
