"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { messageForApiError } from "@/lib/api/errors";
import {
  deliverShoot,
  getShootDelivery,
  getShootDeliveryMessagePreview,
  getShootNotifications,
  markShootPaid,
  regenerateShootLink,
  revokeShootLink,
  type CreateDeliveryLinkResult,
  type DeliveryMessagePreview,
  type ShootDeliveryState,
  type ShootNotificationsResult,
} from "@/lib/api/deliveries";

type Props = {
  shootId: string;
  shootStatus: string;
  priceMinor: number;
  clientEmail?: string | null;
  clientId: string;
};

type SendResult = { status: "sent" | "failed"; errorCode?: string | null } | null;

function emailAvailabilityReason(reason?: string) {
  if (!reason) return "Client email is unavailable.";
  if (reason === "too_many_sends") return "Too many delivery emails were sent for this shoot recently. Try again later.";
  if (reason === "email_daily_limit_reached") return "You've reached today's email limit. Try again tomorrow.";
  return "Client email is unavailable. Add or fix their email address.";
}

export function ShootDeliveryPanel({ shootId, shootStatus, priceMinor, clientEmail, clientId }: Props) {
  const [state, setState] = useState<ShootDeliveryState | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<string | null>(null);
  const [createdLink, setCreatedLink] = useState<CreateDeliveryLinkResult | null>(null);
  const [method, setMethod] = useState<"cash" | "mpesa_direct" | "other">("cash");
  const [note, setNote] = useState("");
  const [sendEmail, setSendEmail] = useState(Boolean(clientEmail));
  const [preview, setPreview] = useState<DeliveryMessagePreview | null>(null);
  const [previewOpen, setPreviewOpen] = useState(false);
  const [messages, setMessages] = useState<ShootNotificationsResult["items"]>([]);
  const [sendResult, setSendResult] = useState<SendResult>(null);

  const emailAvailable = preview?.availability.email.available ?? Boolean(clientEmail);
  const emailReason = preview?.availability.email.reason;

  const load = useCallback(async (showLoading = false) => {
    if (showLoading) setLoading(true);
    const [deliveryRes, previewRes, notificationsRes] = await Promise.all([
      getShootDelivery(shootId),
      getShootDeliveryMessagePreview(shootId),
      getShootNotifications(shootId),
    ]);
    if (showLoading) setLoading(false);
    if (deliveryRes.ok) setState(deliveryRes.data);
    else toast.error(messageForApiError(deliveryRes));
    if (previewRes.ok) {
      setPreview(previewRes.data);
      setSendEmail(previewRes.data.availability.email.available);
    }
    if (notificationsRes.ok) setMessages(notificationsRes.data.items);
  }, [shootId]);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      const [deliveryRes, previewRes, notificationsRes] = await Promise.all([
        getShootDelivery(shootId),
        getShootDeliveryMessagePreview(shootId),
        getShootNotifications(shootId),
      ]);
      if (cancelled) return;
      setLoading(false);
      if (deliveryRes.ok) setState(deliveryRes.data);
      else toast.error(messageForApiError(deliveryRes));
      if (previewRes.ok) {
        setPreview(previewRes.data);
        setSendEmail(previewRes.data.availability.email.available);
      } else {
        setSendEmail(Boolean(clientEmail));
      }
      if (notificationsRes.ok) setMessages(notificationsRes.data.items);
    })();
    return () => { cancelled = true; };
  }, [shootId, clientEmail]);

  async function copyLink(url: string) {
    try {
      await navigator.clipboard.writeText(url);
      toast.success("Link copied");
    } catch {
      toast.error("Could not copy link");
    }
  }

  function applySendResult(res: CreateDeliveryLinkResult) {
    setCreatedLink(res);
    const emailNotification = (res.notifications ?? []).find((n) => n.channel === "email");
    if (emailNotification) {
      setSendResult({ status: emailNotification.status === "sent" ? "sent" : "failed", errorCode: emailNotification.errorCode });
      if (emailNotification.status === "sent") toast.success("Email sent");
      else toast.error("Email could not be sent. The link was still created.");
    } else {
      setSendResult(null);
    }
  }

  async function onDeliver() {
    setBusy("deliver");
    const res = await deliverShoot(shootId, { send: { email: sendEmail && emailAvailable } });
    setBusy(null);
    if (!res.ok) { toast.error(messageForApiError(res)); return; }
    applySendResult(res.data);
    if (!(res.data.notifications ?? []).some((n) => n.channel === "email")) toast.success("Delivery link created");
    await load();
  }

  async function onRegenerate() {
    setBusy("regenerate");
    const res = await regenerateShootLink(shootId, { send: { email: sendEmail && emailAvailable } });
    setBusy(null);
    if (!res.ok) { toast.error(messageForApiError(res)); return; }
    applySendResult(res.data);
    if (!(res.data.notifications ?? []).some((n) => n.channel === "email")) toast.success("New link created");
    await load();
  }

  async function onRevoke() {
    setBusy("revoke");
    const res = await revokeShootLink(shootId);
    setBusy(null);
    if (!res.ok) { toast.error(messageForApiError(res)); return; }
    setCreatedLink(null);
    setSendResult(null);
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
  const emailLabel = useMemo(() => {
    const recipient = preview?.recipientMasked || clientEmail || "client";
    return `Email the link to ${recipient}`;
  }, [preview?.recipientMasked, clientEmail]);

  const sendActionLabel = busy === "deliver" ? "Creating..." : busy === "regenerate" ? "Sending..." : (state?.linkState === "active" || state?.linkState === "revoked" ? "Send again" : "Deliver");

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

            <div className="space-y-3 rounded-lg border p-3">
              <div className="space-y-1">
                <p className="text-sm font-medium">Send to client</p>
                <p className="text-xs text-muted-foreground">Email is sent by Photobooth. “Sent” means the mail server accepted it.</p>
              </div>

              <label className="flex min-h-11 items-start gap-3 text-sm">
                <Checkbox
                  checked={sendEmail && emailAvailable}
                  disabled={!emailAvailable || busy !== null}
                  onCheckedChange={(v) => setSendEmail(Boolean(v))}
                  aria-label={emailLabel}
                />
                <span className={emailAvailable ? "" : "text-muted-foreground"}>
                  <span className="block font-medium">{emailLabel}</span>
                  {!emailAvailable ? (
                    <span className="block text-xs text-muted-foreground">
                      {emailAvailabilityReason(emailReason)}{" "}
                      <Link href={`/dashboard/clients/${clientId}`} className="underline underline-offset-4">Edit client</Link>
                    </span>
                  ) : null}
                </span>
              </label>

              <div className="flex items-center justify-between gap-3 text-xs text-muted-foreground">
                <span>{preview ? `${preview.emailsRemainingToday} emails left today` : ""}</span>
                <button type="button" className="min-h-11 px-2 underline underline-offset-4" onClick={() => setPreviewOpen((v) => !v)} aria-expanded={previewOpen}>
                  {previewOpen ? "Hide preview" : "Preview email"}
                </button>
              </div>

              {previewOpen && preview ? (
                <div className="rounded-md border bg-muted/30 p-3 text-xs">
                  <p><span className="font-medium text-foreground">Subject:</span> {preview.subject}</p>
                  <pre className="mt-2 whitespace-pre-wrap font-sans text-muted-foreground">{preview.bodyText}</pre>
                </div>
              ) : null}

              <div className="flex flex-wrap gap-2">
                {canDeliver ? (
                  <Button type="button" className="min-h-11" disabled={busy !== null} onClick={onDeliver}>{busy === "deliver" ? "Creating..." : "Deliver"}</Button>
                ) : null}
                {state.linkState === "active" || state.linkState === "revoked" ? (
                  <AlertDialog>
                    <AlertDialogTrigger render={<Button type="button" variant="outline" className="min-h-11" disabled={busy !== null} />}>
                      {sendActionLabel}
                    </AlertDialogTrigger>
                    <AlertDialogContent>
                      <AlertDialogHeader>
                        <AlertDialogTitle>Send again?</AlertDialogTitle>
                        <AlertDialogDescription>
                          This will create a new link and the previous link will stop working. {sendEmail && emailAvailable ? "We’ll email the new link to the client." : "You’ll need to copy and share the new link."}
                        </AlertDialogDescription>
                      </AlertDialogHeader>
                      <AlertDialogFooter>
                        <AlertDialogCancel className="min-h-11">Cancel</AlertDialogCancel>
                        <AlertDialogAction className="min-h-11" onClick={onRegenerate}>Send again</AlertDialogAction>
                      </AlertDialogFooter>
                    </AlertDialogContent>
                  </AlertDialog>
                ) : null}
              </div>
            </div>

            {sendResult ? (
              <div className="rounded-lg border bg-muted/30 p-3 text-sm" role="status">
                {sendResult.status === "sent" ? (
                  <p><span className="font-medium">Email sent.</span> The mail server accepted the message.</p>
                ) : (
                  <p><span className="font-medium">Email failed.</span> {messageForApiError({ status: 0, code: sendResult.errorCode ?? undefined, error: "" })} The link was still created.</p>
                )}
              </div>
            ) : null}

            {createdLink ? (
              <div className="rounded-lg border bg-muted/30 p-3 space-y-2">
                <p className="text-sm font-medium">Private link (shown once)</p>
                <p className="text-xs text-muted-foreground break-all">{createdLink.url}</p>
                <div className="flex flex-wrap gap-2">
                  <Button type="button" variant="outline" className="min-h-11" onClick={() => copyLink(createdLink.url)}>Copy link</Button>
                  <Button type="button" variant="outline" className="min-h-11" onClick={() => window.open(`https://wa.me/?text=${encodeURIComponent(`Your photos are ready: ${createdLink.url}`)}`, "_blank", "noopener,noreferrer")}>Share on WhatsApp</Button>
                </div>
              </div>
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

            {messages.length > 0 ? (
              <div className="space-y-2">
                <p className="text-sm font-medium">Messages</p>
                <ul className="space-y-2 text-xs text-muted-foreground">
                  {messages.map((m, i) => (
                    <li key={`${m.createdAt}-${i}`} className="flex items-start justify-between gap-3 rounded-md border p-2">
                      <div>
                        <p className="font-medium text-foreground capitalize">{m.channel} · {m.status}</p>
                        <p>{m.recipient}</p>
                        {m.errorCode ? <p>{messageForApiError({ status: 0, code: m.errorCode, error: "" })}</p> : null}
                      </div>
                      <span className="shrink-0">{new Date(m.createdAt).toLocaleString()}</span>
                    </li>
                  ))}
                </ul>
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
