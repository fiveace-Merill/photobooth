import type { Metadata } from "next";
import { EmptyState } from "@/components/dashboard/empty-state";

export const instant = false;
export const metadata: Metadata = { title: "Settings | Photobooth", robots: { index: false, follow: false } };

export default function SettingsPage() {
  return <EmptyState title="Settings" description="Settings will appear here. Coming soon." />;
}
