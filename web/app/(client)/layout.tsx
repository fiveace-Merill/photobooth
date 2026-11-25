import type { Metadata } from "next";
import "../globals.css";
import { bricolageGrotesque } from "@/lib/fonts";
import { Toaster } from "@/components/ui/sonner";

// Client delivery pages are dark by design. The `dark` class lives on <html>
// so Radix/Base portal content (dialogs, sheets, toasts) inherits dark tokens.
// Trade-off: navigating between this route group and other root layouts causes
// a full page load, but it keeps portal theming correct and isolated.
export const metadata: Metadata = {
  title: "Photo delivery",
};

export default function ClientRootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html
      lang="en"
      className={`${bricolageGrotesque.variable} dark h-full antialiased`}
    >
      <body className="min-h-full bg-background text-foreground">
        {children}
        <Toaster />
      </body>
    </html>
  );
}
