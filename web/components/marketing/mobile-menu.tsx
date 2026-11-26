"use client";

import Link from "next/link";
import { MenuIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import { DEMO_PATH } from "@/lib/site";

const links = [
  { label: "How it works", href: "#how-it-works" },
  { label: "Pricing", href: "#pricing" },
  { label: "FAQ", href: "#faq" },
  { label: "Live demo", href: DEMO_PATH },
  { label: "Log in", href: "/login" },
];

export function MobileMenu() {
  return (
    <Sheet>
      <SheetTrigger
        render={
          <Button variant="outline" size="icon" className="md:hidden min-h-11 min-w-11" aria-label="Open menu" />
        }
      >
        <MenuIcon />
      </SheetTrigger>
      <SheetContent side="right" className="w-[min(20rem,90vw)]">
        <SheetHeader>
          <SheetTitle>Photobooth</SheetTitle>
        </SheetHeader>
        <nav className="flex flex-col gap-1 px-4" aria-label="Mobile">
          {links.map((link) => (
            <SheetClose
              key={link.href}
              render={
                <Link
                  href={link.href}
                  className="inline-flex min-h-11 items-center rounded-md px-2 text-base font-medium text-foreground hover:bg-muted"
                />
              }
            >
              {link.label}
            </SheetClose>
          ))}
          <SheetClose
            render={
              <Link
                href="#early-access"
                className="mt-3 inline-flex min-h-11 items-center justify-center rounded-lg bg-primary px-4 text-sm font-medium text-primary-foreground"
              />
            }
          >
            Get early access
          </SheetClose>
        </nav>
      </SheetContent>
    </Sheet>
  );
}
