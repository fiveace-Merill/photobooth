"use client";

import { useState, type FormEvent } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { adminChangePassword } from "@/lib/api/admin";
import { messageForAdminError } from "@/lib/admin/errors";

export function AccountPasswordForm() {
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [pending, setPending] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setPending(true);
    const res = await adminChangePassword({ current, new: next });
    setPending(false);
    if (!res.ok) { toast.error(messageForAdminError(res)); return; }
    setCurrent(""); setNext("");
    toast.success("Password changed. Other admin sessions were signed out.");
  }

  return (
    <Card className="max-w-lg p-5">
      <h2 className="text-lg font-semibold">Change password</h2>
      <p className="mt-1 text-sm text-muted-foreground">Use your current password. After changing, other admin sessions are signed out.</p>
      <form onSubmit={onSubmit} className="mt-4 space-y-4">
        <div className="space-y-2"><Label htmlFor="current-password">Current password</Label><Input id="current-password" type="password" value={current} onChange={(e) => setCurrent(e.target.value)} className="min-h-11" autoComplete="current-password" required /></div>
        <div className="space-y-2"><Label htmlFor="new-password">New password</Label><Input id="new-password" type="password" value={next} onChange={(e) => setNext(e.target.value)} className="min-h-11" minLength={12} autoComplete="new-password" required /><p className="text-xs text-muted-foreground">At least 12 characters.</p></div>
        <Button type="submit" className="min-h-11" disabled={pending}>{pending ? "Saving..." : "Change password"}</Button>
      </form>
    </Card>
  );
}
