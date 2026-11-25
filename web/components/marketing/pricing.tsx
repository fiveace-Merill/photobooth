import { landing } from "@/lib/content/landing";
import { formatCommission, formatPlanPrice, formatRetention, formatStorage, plans } from "@/lib/content/plans";
import { FadeIn } from "./fade-in";

export function Pricing() {
  return (
    <section id="pricing" className="scroll-mt-20 bg-muted/30" aria-labelledby="pricing-heading">
      <div className="mx-auto max-w-7xl px-4 py-16 sm:px-6 sm:py-20 lg:px-8">
        <FadeIn>
          <h2 id="pricing-heading" className="max-w-3xl text-3xl font-bold tracking-tight sm:text-4xl">
            {landing.pricing.heading}
          </h2>
        </FadeIn>
        <div className="mt-10 grid gap-6 md:grid-cols-2">
          {plans.map((plan) => (
            <FadeIn key={plan.id} className="h-full">
              <article className="flex h-full flex-col rounded-2xl border border-border bg-card p-6 sm:p-8">
                <div className="flex items-start justify-between gap-4">
                  <div>
                    <h3 className="text-2xl font-bold tracking-tight">{plan.name}</h3>
                    <p className="mt-2 text-lg font-medium text-foreground">{formatPlanPrice(plan.monthlyFeeMinor)}</p>
                  </div>
                  {plan.id === "pro" ? (
                    <span className="inline-flex rounded-full bg-volt px-3 py-1 text-xs font-bold text-volt-foreground">
                      More storage
                    </span>
                  ) : null}
                </div>
                <ul className="mt-6 space-y-3 text-sm text-muted-foreground sm:text-base">
                  <li className="border-t border-border pt-3">{formatCommission(plan.commissionBps)}</li>
                  <li className="border-t border-border pt-3">{formatStorage(plan.storageQuotaBytes)}</li>
                  <li className="border-t border-border pt-3">{formatRetention(plan.retentionDays)}</li>
                </ul>
              </article>
            </FadeIn>
          ))}
        </div>
        <p className="mt-6 max-w-2xl text-sm text-muted-foreground">{landing.pricing.note}</p>
      </div>
    </section>
  );
}
