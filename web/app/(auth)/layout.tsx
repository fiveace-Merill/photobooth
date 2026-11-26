import type { Metadata } from "next";
import Link from "next/link";
import "../globals.css";
import { bricolageGrotesque } from "@/lib/fonts";
import { site } from "@/lib/site";

export const metadata: Metadata = {
  title: "Photobooth",
  robots: { index: false, follow: false },
};

export default function AuthRootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" className={`${bricolageGrotesque.variable} h-full antialiased`}>
      <body className="min-h-full bg-background text-foreground">
        <div className="mx-auto flex min-h-screen max-w-6xl flex-col px-4 py-6 sm:px-6 lg:px-8">
          <header>
            <Link href="/" className="inline-flex min-h-11 items-center text-lg font-semibold tracking-tight">
              {site.name}
            </Link>
          </header>
          <div className="flex flex-1 flex-col justify-center py-8">{children}</div>
        </div>
      </body>
    </html>
  );
}
