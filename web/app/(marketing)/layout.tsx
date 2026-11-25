import type { Metadata } from "next";
import "../globals.css";
import { bricolageGrotesque } from "@/lib/fonts";
import { site } from "@/lib/site";

export const metadata: Metadata = {
  metadataBase: new URL(site.url),
  title: {
    default: "Photobooth — Photo delivery for photographers",
    template: "%s — Photobooth",
  },
  description:
    "Photobooth gives every client a private link to preview, approve and pay for their photos with M-PESA. The originals unlock when payment lands.",
  openGraph: {
    type: "website",
    siteName: site.name,
    title: "Photobooth — Photo delivery for photographers",
    description:
      "Photobooth gives every client a private link to preview, approve and pay for their photos with M-PESA. The originals unlock when payment lands.",
    url: "/",
  },
  alternates: {
    canonical: "/",
  },
};

export default function MarketingRootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" className={`${bricolageGrotesque.variable} h-full antialiased`}>
      <body className="min-h-full bg-background text-foreground">
        <a
          href="#main-content"
          className="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-[100] focus:rounded-md focus:bg-primary focus:px-4 focus:py-2 focus:text-primary-foreground"
        >
          Skip to content
        </a>
        {children}
      </body>
    </html>
  );
}
