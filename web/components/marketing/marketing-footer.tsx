import Link from "next/link";
import { DEMO_PATH, site } from "@/lib/site";

const links = [
  { label: "How it works", href: "#how-it-works" },
  { label: "Pricing", href: "#pricing" },
  { label: "FAQ", href: "#faq" },
  { label: "Live demo", href: DEMO_PATH },
  { label: "Privacy", href: "/privacy" },
];

export function MarketingFooter() {
  return (
    <footer className="border-t border-border bg-background">
      <div className="mx-auto flex max-w-7xl flex-col gap-6 px-4 py-10 sm:px-6 lg:flex-row lg:items-center lg:justify-between lg:px-8">
        <div>
          <Link href="/" className="text-base font-semibold tracking-tight text-foreground">
            {site.name}
          </Link>
          <p className="mt-2 text-sm text-muted-foreground">© 2026 Photobooth</p>
        </div>
        <nav aria-label="Footer" className="flex flex-wrap gap-x-5 gap-y-2">
          {links.map((link) => (
            <Link
              key={link.label + link.href}
              href={link.href}
              className="inline-flex min-h-11 items-center text-sm text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              {link.label}
            </Link>
          ))}
        </nav>
      </div>
    </footer>
  );
}
