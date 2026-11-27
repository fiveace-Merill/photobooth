"use client";

import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { messageForApiError } from "@/lib/api/errors";
import {
  deliverShoot,
  getShootDelivery,
  markShootPaid,
  regenerateShootLink,
  revokeShootLink,
  type CreateDeliveryLinkResult,
  type ShootDeliveryState,
} from "@/lib/api/deliveries";

type Props = {
  shootId: string;
  shootStatus: string;
  priceMinor: number;
};

export function ShootDeliveryPanel({ shootId, shootStatus, priceMinor }: Props) {
  const [state, setState] = useState<ShootDeliveryState | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<string | null>(null);
  const [createdLink, setCreatedLink] = useState<CreateDeliveryLinkResult | null>(null);
  const [method, setMethod] = useState<"cash" | "mpesa_direct" | "other">("cash");
  const [note, setNote] = useState("");

  const load = useCallback(async (showLoading = false) => {
    if (showLoading) setLoading(true);
    const res = await getShootDelivery(shootId);
    setLoading(false);
    if (!res.ok) {
      toast.error(messageForApiError(res));
      return;
    }
    setState(res.data);
  }, [shootId]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const res = await getShootDelivery(shootId);
      if (cancelled) return;
      setLoading(false);
      if (!res.ok) {
        toast.error(messageForApiError(res));
        return;
      }
      setState(res.data);
    })();
    return () => { cancelled = true; };
  }, [shootId]);

  async function copyLink(url: string) {
    try {
      await navigator.clipboard.writeText(url);
      toast.success("Link copied");
    } catch {
      toast.error("Could not copy link");
    }
  }

  async function onDeliver() {
    setBusy("deliver");
    const res = await deliverShoot(shootId);
    setBusy(null);
    if (!res.ok) { toast.error(messageForApiError(res)); return; }
    setCreatedLink(res.data);
    toast.success("Delivery link created");
    await load();
  }

  async function onRegenerate() {
    setBusy("regenerate");
    const res = await regenerateShootLink(shootId);
    setBusy(null);
    if (!res.ok) { toast.error(messageForApiError(res)); return; }
    setCreatedLink(res.data);
    toast.success("New link created");
    await load();
  }

  async function onRevoke() {
    setBusy("revoke");
    const res = await revokeShootLink(shootId);
    setBusy(null);
    if (!res.ok) { toast.error(messageForApiError(res)); return; }
    setCreatedLink(null);
    toast.success("Link revoked");
    await load();
  }

  async function onMarkPaid() {
    setBusy("mark-paid");
    const res = await markShootPaid(shootId, { method, note: note.trim() || undefined });
    setBusy(null);
    if (!res.ok) { toast.error(messageForApiError(res)); return; }
    setNote("");
    toast.success("Marked as paid");
    await load();
  }

  const canDeliver = shootStatus === "ready" && state?.linkState !== "active";
  const canMarkPaid = state?.linkState === "active" && !state.unlocked && priceMinor > 0;

  return (
    <Card>
      <CardHeader><CardTitle className="text-base">Delivery</CardTitle></CardHeader>
      <CardContent className="space-y-4">
        {loading ? <p className="text-sm text-muted-foreground">Loading delivery...</p> : null}
        {state ? (
          <>
            <dl className="grid gap-2 text-sm">
              <div className="flex justify-between gap-4"><dt className="text-muted-foreground">Link</dt><dd className="font-medium capitalize">{state.linkState}</dd></div>
              <div className="flex justify-between gap-4"><dt className="text-muted-foreground">Unlocked</dt><dd className="font-medium">{state.unlocked ? "Yes" : "No"}</dd></div>
              {state.expiresAt ? <div className="flex justify-between gap-4"><dt className="text-muted-foreground">Expires</dt><dd className="font-medium">{new Date(state.expiresAt).toLocaleString()}</dd></div> : null}
              {state.lastViewedAt ? <div className="flex justify-between gap-4"><dt className="text-muted-foreground">Last viewed</dt><dd className="font-medium">{new Date(state.lastViewedAt).toLocaleString()}</dd></div> : null}
            </dl>

            {createdLink ? (
              <div className="rounded-lg border bg-muted/30 p-3 space-y-2">
                <p className="text-sm font-medium">Private link (shown once)</p>
                <p className="text-xs text-muted-foreground break-all">{createdLink.url}</p>
                <div className="flex gap-2">
                  <Button type="button" variant="outline" className="min-h-11" onClick={() => copyLink(createdLink.url)}>Copy link</Button>
                </div>
              </div>
            ) : null}

            <div className="flex flex-wrap gap-2">
              {canDeliver ? <Button type="button" className="min-h-11" disabled={busy !== null} onClick={onDeliver}>{busy === "deliver" ? "Creating..." : "Deliver"}</Button> : null}
              {state.linkState === "active" || state.linkState === "revoked" ? (
                <Button type="button" variant="outline" className="min-h-11" disabled={busy !== null} onClick={onRegenerate}>{busy === "regenerate" ? "Regenerating..." : "Regenerate link"}</Button>
              ) : null}
              {state.linkState === "active" ? (
                <AlertDialog>
                  <AlertDialogTrigger render={<Button type="button" variant="outline" className="min-h-11" disabled={busy !== null} />}>Revoke link</AlertDialogTrigger>
                  <AlertDialogContent>
                    <AlertDialogHeader><AlertDialogTitle>Revoke delivery link?</AlertDialogTitle><AlertDialogDescription>The current client link will stop working. You can generate a new one later.</AlertDialogDescription></AlertDialogHeader>
                    <AlertDialogFooter>
                      <AlertDialogCancel className="min-h-11">Cancel</AlertDialogCancel>
                      <AlertDialogAction className="min-h-11" onClick={onRevoke}>Revoke</AlertDialogAction>
                    </AlertDialogFooter>
                  </AlertDialogContent>
                </AlertDialog>
              ) : null}
            </div>

            {canMarkPaid ? (
              <div className="rounded-lg border p-3 space-y-3">
                <div>
                  <p className="text-sm font-medium">Mark as paid manually</p>
                  <p className="text-xs text-muted-foreground">Pilot only. Records outside the commission system; does not create a payment or ledger entry.</p>
                </div>
                <div className="space-y-2">
                  <Label>Method</Label>
                  <Select value={method} onValueChange={(v) => { if (v) setMethod(v as typeof method); }}>
                    <SelectTrigger className="min-h-11"><SelectValue /></SelectTrigger>
                    <SelectContent>
                      <SelectItem value="cash">Cash</SelectItem>
                      <SelectItem value="mpesa_direct">M-PESA direct</SelectItem>
                      <SelectItem value="other">Other</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="mark-paid-note">Note (optional)</Label>
                  <Input id="mark-paid-note" value={note} onChange={(e) => setNote(e.target.value)} className="min-h-11" maxLength={500} />
                </div>
                <Button type="button" className="min-h-11" disabled={busy !== null} onClick={onMarkPaid}>{busy === "mark-paid" ? "Saving..." : "Mark as paid"}</Button>
              </div>
            ) : null}

            {state.events.length > 0 ? (
              <div className="space-y-2">
                <p className="text-sm font-medium">Recent delivery events</p>
                <ul className="space-y-1 text-xs text-muted-foreground">
                  {state.events.map((ev) => (
                    <li key={ev.id} className="flex justify-between gap-3">
                      <span>{ev.kind}</span>
                      <span>{new Date(ev.createdAt).toLocaleString()}</span>
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}
          </>
        ) : null}
      </CardContent>
    </Card>
  );
}
