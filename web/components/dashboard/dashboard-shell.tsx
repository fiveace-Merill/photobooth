import Link from "next/link";
import { MenuIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Sheet, SheetClose, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from "@/components/ui/sheet";
import { LogoutButton } from "@/app/(dashboard)/dashboard/logout-button";
import type { Photographer } from "@/lib/auth/types";

const nav = [
  { href: "/dashboard", label: "Overview" },
  { href: "/dashboard/shoots", label: "Shoots" },
  { href: "/dashboard/clients", label: "Clients" },
  { href: "/dashboard/settings", label: "Settings" },
];

export function DashboardShell({
  photographer,
  children,
}: {
  photographer: Photographer;
  children: React.ReactNode;
}) {
  return (
    <div className="min-h-screen bg-background text-foreground lg:grid lg:grid-cols-[16rem_minmax(0,1fr)]">
      <aside className="hidden border-r border-border lg:flex lg:flex-col">
        <div className="px-6 py-5">
          <Link href="/dashboard" className="text-lg font-semibold tracking-tight">Photobooth</Link>
        </div>
        <nav className="flex-1 px-3" aria-label="Dashboard">
          {nav.map((item) => (
            <Link key={item.href} href={item.href} className="flex min-h-11 items-center rounded-md px-3 text-sm font-medium text-muted-foreground hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
              {item.label}
            </Link>
          ))}
        </nav>
        <div className="border-t border-border px-6 py-5">
          <p className="text-sm font-medium text-foreground">{photographer.business_name}</p>
          <p className="mt-1 text-xs text-muted-foreground">{photographer.email}</p>
          <p className="mt-1 text-xs text-muted-foreground">Plan: {photographer.plan_id}</p>
          <div className="mt-4"><LogoutButton /></div>
        </div>
      </aside>

      <div className="min-w-0">
        <header className="flex items-center justify-between gap-4 border-b border-border px-4 py-3 lg:hidden">
          <Link href="/dashboard" className="text-lg font-semibold tracking-tight">Photobooth</Link>
          <Sheet>
            <SheetTrigger render={<Button variant="outline" size="icon" className="min-h-11 min-w-11" aria-label="Open menu" />}>
              <MenuIcon />
            </SheetTrigger>
            <SheetContent side="left" className="w-[min(20rem,90vw)]">
              <SheetHeader><SheetTitle>Photobooth</SheetTitle></SheetHeader>
              <nav className="flex flex-col gap-1 px-4" aria-label="Mobile dashboard">
                {nav.map((item) => (
                  <SheetClose key={item.href} render={<Link href={item.href} className="flex min-h-11 items-center rounded-md px-2 text-sm font-medium text-muted-foreground hover:bg-muted hover:text-foreground" />}>
                    {item.label}
                  </SheetClose>
                ))}
              </nav>
              <div className="mt-auto border-t border-border px-4 py-5">
                <p className="text-sm font-medium">{photographer.business_name}</p>
                <p className="mt-1 text-xs text-muted-foreground">{photographer.email}</p>
                <p className="mt-1 text-xs text-muted-foreground">Plan: {photographer.plan_id}</p>
                <div className="mt-4"><LogoutButton /></div>
              </div>
            </SheetContent>
          </Sheet>
        </header>
        <main className="px-4 py-8 sm:px-6 lg:px-8">{children}</main>
      </div>
    </div>
  );
}
