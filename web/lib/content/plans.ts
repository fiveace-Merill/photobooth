// PLACEHOLDER values pending the photographer survey.
// Mirrors migrations/00001_tenants_and_auth.sql seed plans. Amounts in minor
// units (KES cents), commission in basis points (bps), never floats.
import { formatMoney } from "@/lib/money";

export type Plan = {
  id: "free" | "pro";
  name: string;
  monthlyFeeMinor: number;
  commissionBps: number;
  storageQuotaBytes: number;
  retentionDays: number;
};

export const plans: Plan[] = [
  {
    id: "free",
    name: "Free",
    monthlyFeeMinor: 0,
    commissionBps: 800,
    storageQuotaBytes: 5_368_709_120,
    retentionDays: 30,
  },
  {
    id: "pro",
    name: "Pro",
    monthlyFeeMinor: 100_000,
    commissionBps: 300,
    storageQuotaBytes: 107_374_182_400,
    retentionDays: 180,
  },
];

export function formatPlanPrice(monthlyFeeMinor: number): string {
  if (monthlyFeeMinor === 0) return "KES 0 / month";
  return `${formatMoney(monthlyFeeMinor, "KES")} / month`;
}

export function formatCommission(bps: number): string {
  const percent = bps / 100;
  return `${Number.isInteger(percent) ? percent.toFixed(0) : percent.toString()}% per paid delivery`;
}

export function formatStorage(bytes: number): string {
  const gb = bytes / (1024 * 1024 * 1024);
  return `${gb >= 100 ? gb.toFixed(0) : gb.toString()} GB storage`;
}

export function formatRetention(days: number): string {
  return `${days} days photo retention`;
}
