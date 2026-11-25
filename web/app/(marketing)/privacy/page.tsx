import type { Metadata } from "next";
import Link from "next/link";
import { MarketingFooter } from "@/components/marketing/marketing-footer";
import { MarketingHeader } from "@/components/marketing/marketing-header";
import { site } from "@/lib/site";

export const metadata: Metadata = {
  title: "Privacy",
  description: "Draft privacy policy for Photobooth early access.",
  alternates: { canonical: "/privacy" },
  openGraph: {
    title: "Privacy — Photobooth",
    description: "Draft privacy policy for Photobooth early access.",
    url: "/privacy",
    type: "website",
  },
};

export default function PrivacyPage() {
  return (
    <>
      <MarketingHeader />
      <main id="main-content" className="mx-auto max-w-3xl px-4 py-16 sm:px-6 sm:py-20 lg:px-8">
        <p className="inline-flex rounded-full bg-volt px-3 py-1 text-sm font-semibold text-volt-foreground">
          DRAFT pending legal review
        </p>
        <h1 className="mt-5 text-4xl font-bold tracking-tight sm:text-5xl">Privacy</h1>
        <p className="mt-6 text-base leading-relaxed text-muted-foreground">
          This is a short, plain-language draft for early access. It is not legal advice and will be
          reviewed before public launch.
        </p>

        <div className="mt-10 space-y-8 text-sm leading-relaxed text-muted-foreground sm:text-base">
          <section>
            <h2 className="text-xl font-semibold text-foreground">What we collect</h2>
            <p className="mt-3">
              When you request early access, we collect the information you submit: your name, contact
              details, photography type, shoots per month, and consent to be contacted. We also collect
              basic technical information needed to operate the website securely.
            </p>
          </section>
          <section>
            <h2 className="text-xl font-semibold text-foreground">How we use it</h2>
            <p className="mt-3">
              We use early-access information to contact you about Photobooth, understand photographer
              needs, and plan invites. We do not sell your personal information.
            </p>
          </section>
          <section>
            <h2 className="text-xl font-semibold text-foreground">Client photos</h2>
            <p className="mt-3">
              Photobooth is designed so photographers control their client deliveries. Clients should
              only be able to access their own private shoot links. Photo storage, retention, and
              deletion rules will be described clearly in the final product terms and privacy policy.
            </p>
          </section>
          <section>
            <h2 className="text-xl font-semibold text-foreground">Your choices</h2>
            <p className="mt-3">
              You can ask us to update or remove your early-access information by contacting{" "}
              <a href={`mailto:${site.contactEmail}`} className="text-foreground underline underline-offset-4">
                {site.contactEmail}
              </a>
              .
            </p>
          </section>
          <section>
            <h2 className="text-xl font-semibold text-foreground">Changes</h2>
            <p className="mt-3">
              We will update this draft as the product and legal review evolve before public launch.
            </p>
          </section>
        </div>

        <p className="mt-10">
          <Link href="/" className="inline-flex min-h-11 items-center text-sm font-medium text-foreground underline underline-offset-4">
            Back to home
          </Link>
        </p>
      </main>
      <MarketingFooter />
    </>
  );
}
