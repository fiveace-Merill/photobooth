import type { Metadata } from "next";
import { Suspense } from "react";
import { SignupForm } from "./signup-form";

export const metadata: Metadata = {
  robots: { index: false, follow: false },
  referrer: "no-referrer",
};

async function SignupPageContent({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const sp = await searchParams;
  const inviteRaw = sp.invite;
  const inviteToken = typeof inviteRaw === "string" ? inviteRaw : undefined;
  return <SignupForm inviteToken={inviteToken} />;
}

export default function SignupPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  return (
    <Suspense
      fallback={
        <main className="mx-auto w-full max-w-md">
          <p className="text-sm font-medium text-muted-foreground">Photobooth</p>
          <h1 className="mt-3 text-4xl font-semibold tracking-tight">Create your account</h1>
          <div className="mt-8 rounded-xl border border-border bg-card p-5 text-sm text-muted-foreground" role="status">
            Checking signup access...
          </div>
        </main>
      }
    >
      <SignupPageContent searchParams={searchParams} />
    </Suspense>
  );
}
