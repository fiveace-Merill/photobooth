"use client";

import { landing } from "@/lib/content/landing";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";

export function Faq() {
  return (
    <section id="faq" className="scroll-mt-20 bg-muted/30" aria-labelledby="faq-heading">
      <div className="mx-auto max-w-4xl px-4 py-16 sm:px-6 sm:py-20 lg:px-8">
        <h2 id="faq-heading" className="text-3xl font-bold tracking-tight sm:text-4xl">
          {landing.faq.heading}
        </h2>
        <Accordion className="mt-8 w-full">
          {landing.faq.items.map((item, i) => (
            <AccordionItem key={item.q} value={`faq-${i}`}>
              <AccordionTrigger className="min-h-11 py-4 text-base font-medium text-foreground hover:no-underline sm:text-lg">
                {item.q}
              </AccordionTrigger>
              <AccordionContent className="text-sm leading-relaxed text-muted-foreground sm:text-base">
                {item.a}
              </AccordionContent>
            </AccordionItem>
          ))}
        </Accordion>
      </div>
    </section>
  );
}
