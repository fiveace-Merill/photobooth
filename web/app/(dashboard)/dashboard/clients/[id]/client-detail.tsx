"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from "@/components/ui/alert-dialog";
import { deleteClient, updateClient, type Client } from "@/lib/api/clients";
import type { Shoot } from "@/lib/api/shoots";
import { StatusChip } from "@/components/dashboard/status-chip";
import { OverdueLabel } from "@/components/dashboard/overdue-label";
import { messageForApiError } from "@/lib/api/errors";
import { formatKenyanPhone } from "@/lib/phone";
import { formatMoney } from "@/lib/money";
import { formatDisplayDate } from "@/lib/dates";
import { useUnsavedChanges } from "@/components/dashboard/use-unsaved-changes";

export function ClientDetail({ client, shoots }: { client: Client; shoots: Shoot[] }) {
  const router = useRouter();
  const [name, setName] = useState(client.name);
  const [phone, setPhone] = useState(client.phone ? formatKenyanPhone(client.phone) : "");
  const [email, setEmail] = useState(client.email ?? "");
  const [notes, setNotes] = useState(client.notes ?? "");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const dirty = name !== client.name || phone !== (client.phone ? formatKenyanPhone(client.phone) : "") || email !== (client.email ?? "") || notes !== (client.notes ?? "");
  useUnsavedChanges(dirty && !saving);

  async function onSave(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setSaving(true);
    const res = await updateClient(client.id, { name, phone: phone || undefined, email: email || undefined, notes: notes || undefined });
    setSaving(false);
    if (!res.ok) { setError(messageForApiError(res)); return; }
    toast.success("Client updated");
    router.refresh();
  }

  async function onDelete() {
    const res = await deleteClient(client.id);
    if (!res.ok) { toast.error(messageForApiError(res)); return; }
    toast.success("Client deleted");
    router.push("/dashboard/clients");
    router.refresh();
  }

  const hasShoots = shoots.length > 0;

  return (
    <div className="space-y-6 max-w-4xl">
      <div><h1 className="text-2xl font-semibold tracking-tight">{client.name}</h1><p className="text-sm text-muted-foreground">{client.phone ? formatKenyanPhone(client.phone) : "No phone"}{client.email ? ` · ${client.email}` : ""}</p></div>
      <div className="grid gap-6 lg:grid-cols-[1fr_20rem]">
        <Card>
          <CardHeader><CardTitle className="text-base">Edit client</CardTitle></CardHeader>
          <CardContent>
            <form className="space-y-4" onSubmit={onSave}>
              <div className="space-y-2"><Label htmlFor="name">Name</Label><Input id="name" value={name} onChange={(e) => setName(e.target.value)} className="min-h-11" /></div>
              <div className="space-y-2"><Label htmlFor="phone">Phone</Label><Input id="phone" value={phone} onChange={(e) => setPhone(e.target.value)} className="min-h-11" placeholder="0712 345 678" /></div>
              <div className="space-y-2"><Label htmlFor="email">Email</Label><Input id="email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} className="min-h-11" /></div>
              <div className="space-y-2"><Label htmlFor="notes">Notes</Label><Textarea id="notes" value={notes} onChange={(e) => setNotes(e.target.value)} /></div>
              {error ? <p className="text-sm text-destructive" role="status">{error}</p> : null}
              <Button type="submit" className="min-h-11" disabled={saving || !dirty}>{saving ? "Saving..." : "Save changes"}</Button>
            </form>
          </CardContent>
        </Card>

        <Card>
          <CardHeader><CardTitle className="text-base">Danger zone</CardTitle></CardHeader>
          <CardContent className="space-y-3">
            {hasShoots ? <p className="text-sm text-muted-foreground">This client has shoots and cannot be deleted until those shoots are removed.</p> : null}
            <AlertDialog>
              <AlertDialogTrigger render={<Button variant="destructive" className="min-h-11 w-full" disabled={hasShoots} />}>Delete client</AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader><AlertDialogTitle>Delete client?</AlertDialogTitle><AlertDialogDescription>This will permanently delete “{client.name}”. This cannot be undone.</AlertDialogDescription></AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel className="min-h-11">Cancel</AlertDialogCancel>
                  <AlertDialogAction className="min-h-11 bg-destructive text-white hover:bg-destructive/90" onClick={onDelete}>Delete</AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </CardContent>
        </Card>
      </div>

      <section className="space-y-3">
        <h2 className="text-lg font-semibold tracking-tight">Shoots</h2>
        {shoots.length === 0 ? <Card><CardContent className="p-5"><p className="text-sm text-muted-foreground">No shoots for this client yet.</p></CardContent></Card> : (
          <div className="space-y-3">{shoots.map((shoot) => (
            <Card key={shoot.id}><CardContent className="p-4 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
              <div><Link href={`/dashboard/shoots/${shoot.id}`} className="font-medium hover:underline">{shoot.title}</Link><p className="text-sm text-muted-foreground">{shoot.shootDate ? formatDisplayDate(shoot.shootDate) : "No shoot date"} · {formatMoney(shoot.priceMinor, shoot.currency)}</p></div>
              <div className="flex items-center gap-2"><StatusChip status={shoot.status} />{shoot.overdue ? <OverdueLabel daysOverdue={shoot.daysOverdue} /> : null}</div>
            </CardContent></Card>
          ))}</div>
        )}
      </section>
    </div>
  );
}
