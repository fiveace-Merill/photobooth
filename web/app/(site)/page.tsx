export default function Home() {
  return (
    <main className="flex min-h-screen flex-col items-center justify-center px-6 py-24 text-center">
      <p className="text-sm font-medium tracking-wide text-muted-foreground">Photobooth</p>
      <h1 className="mt-4 max-w-2xl text-4xl font-semibold tracking-tight sm:text-5xl">
        Photo delivery for photographers and their clients.
      </h1>
      <p className="mt-6 max-w-xl text-lg text-muted-foreground">
        The client delivery preview is available at <code className="rounded bg-muted px-1.5 py-0.5">/g/demo</code>.
      </p>
      <div className="mt-8 flex gap-3">
        <a href="/signup" className="inline-flex min-h-11 items-center justify-center rounded-lg bg-primary px-5 text-sm font-medium text-primary-foreground">Sign up</a>
        <a href="/login" className="inline-flex min-h-11 items-center justify-center rounded-lg border border-border px-5 text-sm font-medium">Log in</a>
      </div>
    </main>
  );
}
