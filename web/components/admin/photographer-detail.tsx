"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { AdminStatusChip } from "./status-chip";
import { photographerStatusChip } from "@/lib/admin/status";
import { messageForAdminError } from "@/lib/admin/errors";
import { adminChangePhotographerPlan, adminSuspendPhotographer, adminUnsuspendPhotographer, type AdminPhotographer } from "@/lib/api/admin";

export function PhotographerDetail({ initial }: { initial: AdminPhotographer }) {
  const router = useRouter();
  const [p, setP] = useState(initial);
  const [reason, setReason] = useState("");
  const [planId, setPlanId] = useState(p.planId);
  const [pending, setPending] = useState<string | null>(null);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">{p.businessName}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{p.email} · {p.slug}</p>
      </div>
      <Card className="p-5 space-y-3">
        <div className="flex items-center gap-3"><AdminStatusChip chip={photographerStatusChip(p.status)} /><span className="text-sm text-muted-foreground">Plan: {p.planId}</span></div>
        <dl className="grid gap-3 text-sm sm:grid-cols-2">
          <div><dt className="text-muted-foreground">Storage</dt><dd className="font-medium">{formatBytes(p.storageUsedBytes)} / {formatBytes(p.storageQuotaBytes)}</dd></div>
          <div><dt className="text-muted-foreground">Created</dt><dd className="font-medium">{new Date(p.createdAt).toLocaleString()}</dd></div>
          <div><dt className="text-muted-foreground">Last login</dt><dd className="font-medium">{p.lastLoginAt ? new Date(p.lastLoginAt).toLocaleString() : "Never"}</dd></div>
          <div><dt className="text-muted-foreground">Usage</dt><dd className="font-medium">{p.shoots} shoots · {p.photos} photos · {p.delivered} delivered</dd></div>
          {p.suspendedReason ? <div className="sm:col-span-2"><dt className="text-muted-foreground">Suspension reason</dt><dd className="font-medium">{p.suspendedReason}</dd></div> : null}
        </dl>
      </Card>

      <Card className="p-5 space-y-4">
        <h2 className="text-lg font-semibold">Change plan</h2>
        <p className="text-sm text-muted-foreground">Changing to a smaller quota does not delete existing data.</p>
        <div className="max-w-xs space-y-2">
          <Label>Plan</Label>
          <Select value={planId} onValueChange={(v) => setPlanId(v ?? p.planId)}>
            <SelectTrigger className="min-h-11 w-full"><SelectValue /></SelectTrigger>
            <SelectContent><SelectItem value="free">Free</SelectItem><SelectItem value="starter">Starter</SelectItem><SelectItem value="pro">Pro</SelectItem></SelectContent>
          </Select>
        </div>
        <Button className="min-h-11" disabled={pending!==null || planId===p.planId} onClick={async () => {
          setPending("plan");
          const res = await adminChangePhotographerPlan(p.id, planId);
          setPending(null);
          if (!res.ok) { toast.error(messageForAdminError(res)); return; }
          toast.success("Plan updated");
          setP({ ...p, planId });
          router.refresh();
        }}>{pending==="plan" ? "Saving..." : "Save plan"}</Button>
      </Card>

      <Card className="p-5 space-y-4 border-destructive/30">
        <h2 className="text-lg font-semibold">{p.status === "suspended" ? "Unsuspend photographer" : "Suspend photographer"}</h2>
        {p.status === "suspended" ? (
          <>
            <p className="text-sm text-muted-foreground">Unsuspending restores the account. The photographer will be able to log in again.</p>
            <Button className="min-h-11" disabled={pending!==null} onClick={async () => {
              setPending("unsuspend");
              const res = await adminUnsuspendPhotographer(p.id);
              setPending(null);
              if (!res.ok) { toast.error(messageForAdminError(res)); return; }
              toast.success("Photographer unsuspended");
              setP({ ...p, status: "active", suspendedReason: "", suspendedAt: null });
              router.refresh();
            }}>{pending==="unsuspend"?"Unsuspending...":"Unsuspend"}</Button>
          </>
        ) : (
          <>
            <p className="text-sm text-muted-foreground">Suspending deletes the photographer&apos;s sessions and blocks login with a generic error. Existing sessions stop working immediately. Existing delivery links keep working until they expire.</p>
            <div className="space-y-2"><Label htmlFor="suspend-reason">Reason <span aria-hidden="true">*</span></Label><Input id="suspend-reason" value={reason} onChange={(e) => setReason(e.target.value)} className="min-h-11" placeholder="Reason for suspension" /></div>
            <Button variant="destructive" className="min-h-11" disabled={pending!==null || !reason.trim()} onClick={async () => {
              setPending("suspend");
              const res = await adminSuspendPhotographer(p.id, reason.trim());
              setPending(null);
              if (!res.ok) { toast.error(messageForAdminError(res)); return; }
              toast.success("Photographer suspended");
              setP({ ...p, status: "suspended", suspendedReason: reason.trim() });
              setReason("");
              router.refresh();
            }}>{pending==="suspend"?"Suspending...":"Suspend photographer"}</Button>
          </>
        )}
      </Card>
    </div>
  );
}

function formatBytes(bytes: number): string { if (!bytes) return "0 B"; const u=["B","KB","MB","GB","TB"]; let i=0,v=bytes; while(v>=1024&&i<u.length-1){v/=1024;i++;} return `${v>=10||i===0?v.toFixed(0):v.toFixed(1)} ${u[i]}`; }
