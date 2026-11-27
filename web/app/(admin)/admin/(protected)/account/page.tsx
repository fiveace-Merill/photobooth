import { AccountPasswordForm } from "@/components/admin/account-password-form";

export const instant = false;

export default function AdminAccountPage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Account</h1>
        <p className="mt-1 text-sm text-muted-foreground">Manage your admin account security.</p>
      </div>
      <AccountPasswordForm />
    </div>
  );
}
