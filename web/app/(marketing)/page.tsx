import type { Metadata } from "next";
import { EarlyAccess } from "@/components/marketing/early-access";
import { Faq } from "@/components/marketing/faq";
import { Features } from "@/components/marketing/features";
import { Hero } from "@/components/marketing/hero";
import { HowItWorks } from "@/components/marketing/how-it-works";
import { MarketingFooter } from "@/components/marketing/marketing-footer";
import { MarketingHeader } from "@/components/marketing/marketing-header";
import { Pricing } from "@/components/marketing/pricing";
import { Problems } from "@/components/marketing/problems";
import { Testimonials } from "@/components/marketing/testimonials";

export const metadata: Metadata = {
  title: "Photo delivery for photographers",
  description:
    "Photobooth gives every client a private link to preview, approve and pay for their photos with M-PESA. The originals unlock when payment lands.",
  alternates: { canonical: "/" },
  openGraph: {
    title: "Photobooth — Photo delivery for photographers",
    description:
      "Photobooth gives every client a private link to preview, approve and pay for their photos with M-PESA. The originals unlock when payment lands.",
    url: "/",
    type: "website",
  },
};

export default function MarketingPage() {
  return (
    <>
      <MarketingHeader />
      <main id="main-content">
        <Hero />
        <Problems />
        <HowItWorks />
        <Features />
        <Pricing />
        <Testimonials />
        <Faq />
        <EarlyAccess />
      </main>
      <MarketingFooter />
    </>
  );
}
