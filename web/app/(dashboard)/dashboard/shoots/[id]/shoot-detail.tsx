"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from "@/components/ui/alert-dialog";
import { StatusChip } from "@/components/dashboard/status-chip";
import { OverdueLabel } from "@/components/dashboard/overdue-label";
import { deleteShoot, setShootStatus, updateShoot, type Shoot } from "@/lib/api/shoots";
import { ShootPhotosPanel } from "@/components/dashboard/photos/shoot-photos-panel";
import { ShootDeliveryPanel } from "@/components/dashboard/shoots/shoot-delivery-panel";
import type { Client } from "@/lib/api/clients";
import { messageForApiError } from "@/lib/api/errors";
import { formatKesInput, formatMoney, parseKesToMinor } from "@/lib/money";
import { formatKenyanPhone } from "@/lib/phone";
import { useUnsavedChanges } from "@/components/dashboard/use-unsaved-changes";

const SETTABLE: Array<"shot" | "editing" | "ready"> = ["shot", "editing", "ready"];

export function ShootDetail({ shoot, client }: { shoot: Shoot; client: Client | null }) {
  const router = useRouter();
  const [title, setTitle] = useState(shoot.title);
  const [shootDate, setShootDate] = useState(shoot.shootDate ?? "");
  const [dueDate, setDueDate] = useState(shoot.dueDate ?? "");
  const [price, setPrice] = useState(formatKesInput(shoot.priceMinor));
  const [saving, setSaving] = useState(false);
  const [statusSaving, setStatusSaving] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const dirty = title !== shoot.title || shootDate !== (shoot.shootDate ?? "") || dueDate !== (shoot.dueDate ?? "") || price !== formatKesInput(shoot.priceMinor);
  useUnsavedChanges(dirty && !saving);

  async function onSave(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    let priceMinor = 0;
    try { priceMinor = parseKesToMinor(price); } catch { setError("Enter a valid whole KES price, like 3,000."); return; }
    setSaving(true);
    const res = await updateShoot(shoot.id, { title, shootDate: shootDate || undefined, dueDate: dueDate || undefined, priceMinor, currency: "KES" });
    setSaving(false);
    if (!res.ok) { setError(messageForApiError(res)); return; }
    toast.success("Shoot updated");
    router.refresh();
  }

  async function onStatus(status: "shot" | "editing" | "ready") {
    setStatusSaving(status);
    const res = await setShootStatus(shoot.id, status);
    setStatusSaving(null);
    if (!res.ok) { toast.error(messageForApiError(res)); return; }
    toast.success("Status updated");
    router.refresh();
  }

  async function onDelete() {
    const res = await deleteShoot(shoot.id);
    if (!res.ok) { toast.error(messageForApiError(res)); return; }
    toast.success("Shoot deleted");
    router.push("/dashboard/shoots");
    router.refresh();
  }

  return (
    <div className="space-y-6 max-w-4xl">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">{shoot.title}</h1>
          <p className="text-sm text-muted-foreground">{shoot.clientName ?? "Client"} · {formatMoney(shoot.priceMinor, shoot.currency)}</p>
        </div>
        <div className="flex items-center gap-2"><StatusChip status={shoot.status} />{shoot.overdue ? <OverdueLabel daysOverdue={shoot.daysOverdue} /> : null}</div>
      </div>

      <Card>
        <CardHeader><CardTitle className="text-base">Status</CardTitle></CardHeader>
        <CardContent className="space-y-3">
          <div className="flex flex-wrap gap-2">
            {SETTABLE.map((s) => (
              <Button key={s} type="button" variant={shoot.status === s ? "secondary" : "outline"} className="min-h-11" disabled={statusSaving !== null} onClick={() => onStatus(s)}>
                {statusSaving === s ? "Saving..." : s[0].toUpperCase() + s.slice(1)}
              </Button>
            ))}
            {shoot.status === "delivered" || shoot.status === "expired" ? (
              <div className="flex items-center gap-2 text-sm text-muted-foreground"><StatusChip status={shoot.status} /><span>This status is set by the system and cannot be changed here.</span></div>
            ) : null}
          </div>
        </CardContent>
      </Card>

      <div className="grid gap-6 lg:grid-cols-[1fr_20rem]">
        <div className="space-y-6">
          <Card>
            <CardHeader><CardTitle className="text-base">Edit shoot</CardTitle></CardHeader>
            <CardContent>
              <form className="space-y-4" onSubmit={onSave}>
                <div className="space-y-2"><Label htmlFor="title">Title</Label><Input id="title" value={title} onChange={(e) => setTitle(e.target.value)} className="min-h-11" /></div>
                <div className="grid gap-4 sm:grid-cols-2">
                  <div className="space-y-2"><Label htmlFor="shootDate">Shoot date</Label><Input id="shootDate" type="date" value={shootDate} onChange={(e) => setShootDate(e.target.value)} className="min-h-11" /></div>
                  <div className="space-y-2"><Label htmlFor="dueDate">Due date</Label><Input id="dueDate" type="date" value={dueDate} onChange={(e) => setDueDate(e.target.value)} className="min-h-11" /></div>
                </div>
                <div className="space-y-2"><Label htmlFor="price">Price (KES)</Label><Input id="price" inputMode="numeric" value={price} onChange={(e) => setPrice(e.target.value)} className="min-h-11" /><p className="text-xs text-muted-foreground">Enter whole KES, for example 3,000. Use 0 for free delivery.</p></div>
                {error ? <p className="text-sm text-destructive" role="status">{error}</p> : null}
                <Button type="submit" className="min-h-11" disabled={saving || !dirty}>{saving ? "Saving..." : "Save changes"}</Button>
              </form>
            </CardContent>
          </Card>

          <ShootPhotosPanel shootId={shoot.id} shootExpired={shoot.status === "expired"} />
        </div>

        <div className="space-y-6">
          <ShootDeliveryPanel shootId={shoot.id} shootStatus={shoot.status} priceMinor={shoot.priceMinor} />
          <Card>
            <CardHeader><CardTitle className="text-base">Client</CardTitle></CardHeader>
            <CardContent className="space-y-2 text-sm">
              <p className="font-medium">{client?.name ?? shoot.clientName ?? "Client"}</p>
              {client?.phone ? <p className="text-muted-foreground">{formatKenyanPhone(client.phone)}</p> : null}
              {client?.email ? <p className="text-muted-foreground">{client.email}</p> : null}
              <Link href={`/dashboard/clients/${shoot.clientId}`} className="inline-flex min-h-11 items-center font-medium underline underline-offset-4">View client</Link>
            </CardContent>
          </Card>

          <Card>
            <CardHeader><CardTitle className="text-base">Danger zone</CardTitle></CardHeader>
            <CardContent>
              <AlertDialog>
                <AlertDialogTrigger render={<Button variant="destructive" className="min-h-11 w-full" />}>Delete shoot</AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader><AlertDialogTitle>Delete shoot?</AlertDialogTitle><AlertDialogDescription>This will permanently delete “{shoot.title}”. This cannot be undone.</AlertDialogDescription></AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel className="min-h-11">Cancel</AlertDialogCancel>
                    <AlertDialogAction className="min-h-11 bg-destructive text-white hover:bg-destructive/90" onClick={onDelete}>Delete</AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
