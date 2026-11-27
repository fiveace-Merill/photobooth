"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { MenuIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Sheet, SheetClose, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from "@/components/ui/sheet";
import { adminLogout, type AdminMe } from "@/lib/api/admin";
import { toast } from "sonner";

const nav = [
  { href: "/admin", label: "Overview" },
  { href: "/admin/waitlist", label: "Waitlist" },
  { href: "/admin/invitations", label: "Invitations" },
  { href: "/admin/photographers", label: "Photographers" },
  { href: "/admin/audit", label: "Audit log" },
  { href: "/admin/account", label: "Account" },
];

export function AdminShell({ admin, children }: { admin: AdminMe; children: React.ReactNode }) {
  return (
    <div className="min-h-screen bg-background text-foreground lg:grid lg:grid-cols-[16rem_minmax(0,1fr)]">
      <aside className="hidden border-r border-border lg:flex lg:flex-col">
        <div className="px-6 py-5">
          <Link href="/admin" className="text-lg font-semibold tracking-tight">Photobooth Admin</Link>
        </div>
        <nav className="flex-1 px-3" aria-label="Admin navigation">
          <ul className="space-y-1">
            {nav.map((item) => <li key={item.href}><NavLink href={item.href} label={item.label} /></li>)}
          </ul>
        </nav>
        <div className="border-t border-border px-4 py-4 text-sm">
          <p className="truncate font-medium">{admin.email}</p>
          <p className="truncate text-muted-foreground">Platform admin</p>
          <div className="mt-3"><LogoutButton /></div>
        </div>
      </aside>

      <div className="min-w-0">
        <header className="flex items-center justify-between border-b border-border px-4 py-3 lg:hidden">
          <Link href="/admin" className="text-lg font-semibold tracking-tight">Photobooth Admin</Link>
          <Sheet>
            <SheetTrigger render={<Button variant="outline" size="icon" className="min-h-11 min-w-11" aria-label="Open navigation menu"><MenuIcon /></Button>} />
            <SheetContent side="left" className="w-[min(20rem,90vw)]">
              <SheetHeader><SheetTitle>Admin</SheetTitle></SheetHeader>
              <nav className="px-4" aria-label="Admin navigation">
                <ul className="space-y-1">
                  {nav.map((item) => (
                    <li key={item.href}>
                      <SheetClose render={<Link href={item.href} className="block rounded-md px-3 py-3 text-sm text-muted-foreground hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring" />}>
                        {item.label}
                      </SheetClose>
                    </li>
                  ))}
                </ul>
              </nav>
              <div className="mt-auto border-t border-border px-4 py-4 text-sm">
                <p className="truncate font-medium">{admin.email}</p>
                <div className="mt-3"><LogoutButton /></div>
              </div>
            </SheetContent>
          </Sheet>
        </header>
        <main className="px-4 py-6 sm:px-6 lg:px-8">{children}</main>
      </div>
    </div>
  );
}

function NavLink({ href, label }: { href: string; label: string }) {
  const pathname = usePathname();
  const active = pathname === href || (href !== "/admin" && pathname.startsWith(href));
  return (
    <Link
      href={href}
      className={`block rounded-md px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${active ? "bg-muted text-foreground font-medium" : "text-muted-foreground hover:bg-muted hover:text-foreground"}`}
      aria-current={active ? "page" : undefined}
    >
      {label}
    </Link>
  );
}

function LogoutButton() {
  const router = useRouter();
  return (
    <Button
      variant="outline"
      className="min-h-11 w-full"
      onClick={async () => {
        await adminLogout();
        toast.success("Signed out");
        router.push("/admin/login");
        router.refresh();
      }}
    >
      Log out
    </Button>
  );
}
