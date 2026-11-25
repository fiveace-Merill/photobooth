export default function DeliveryNotFound() {
  return (
    <main className="mx-auto flex min-h-screen max-w-[1200px] flex-col justify-center px-4 py-16 sm:px-6">
      <p className="text-sm font-medium text-muted-foreground">Photobooth</p>
      <h1 className="mt-4 max-w-xl text-4xl font-semibold tracking-tight sm:text-5xl">
        This link isn&apos;t valid or has expired
      </h1>
      <p className="mt-4 max-w-lg text-base text-muted-foreground">
        Check the link you were sent, or ask the photographer to send a new one.
      </p>
    </main>
  );
}
