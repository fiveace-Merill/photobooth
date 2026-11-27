"use client";

import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { adminInviteWaitlist, type WaitlistEntry } from "@/lib/api/admin";
import { messageForAdminError } from "@/lib/admin/errors";
import { canSubmitInvite, getInviteDialogState } from "@/lib/admin/invite";

export function InviteDialog({ entry, onInvited }: { entry: WaitlistEntry; onInvited: (e: WaitlistEntry) => void }) {
  const state = getInviteDialogState(entry, true);
  const [open, setOpen] = useState(false);
  const [email, setEmail] = useState(state.email);
  const [planId, setPlanId] = useState("free");
  const [days, setDays] = useState(7);
  const [send, setSend] = useState(false);
  const [pending, setPending] = useState(false);
  const [createdUrl, setCreatedUrl] = useState<string | null>(null);
  const [emailStatus, setEmailStatus] = useState<string | null>(null);

  const canSubmit = canSubmitInvite({ email, planId, days });

  return (
    <Dialog open={open} onOpenChange={(v) => { setOpen(v); if (!v) { setCreatedUrl(null); setEmailStatus(null); setEmail(state.email); setSend(false); } }}>
      <DialogTrigger render={<Button className="min-h-11">Invite</Button>} />
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Invite to Photobooth</DialogTitle>
        </DialogHeader>
        {createdUrl ? (
          <div className="space-y-4">
            <p className="text-sm text-muted-foreground">Share this invitation link now. It cannot be shown again after you close this dialog.</p>
            <div className="space-y-2">
              <Label htmlFor="invite-url">Invitation link</Label>
              <Input id="invite-url" readOnly value={createdUrl} className="min-h-11 font-mono text-xs" />
            </div>
            <div className="flex flex-wrap gap-2">
              <Button variant="outline" className="min-h-11" onClick={() => navigator.clipboard.writeText(createdUrl)}>Copy link</Button>
              <Button variant="outline" className="min-h-11" onClick={() => window.open(`https://wa.me/?text=${encodeURIComponent(`You've been invited to Photobooth: ${createdUrl}`)}`, "_blank", "noopener,noreferrer")}>Share on WhatsApp</Button>
            </div>
            {emailStatus ? <p className="text-sm text-muted-foreground">Email status: {emailStatus}</p> : null}
            <p className="text-sm text-destructive" role="status">This link is shown once. Copy it before closing.</p>
          </div>
        ) : (
          <div className="space-y-4">
            {state.note ? <p className="rounded-md border border-border bg-muted/30 p-3 text-sm text-muted-foreground">{state.note}</p> : null}
            <div className="space-y-2">
              <Label htmlFor="invite-email">Email {state.needsEmail ? <span aria-hidden="true">*</span> : null}</Label>
              <Input id="invite-email" value={email} onChange={(e) => setEmail(e.target.value)} className="min-h-11" placeholder="name@example.com" />
            </div>
            <div className="space-y-2">
              <Label>Plan</Label>
              <Select value={planId} onValueChange={(v) => setPlanId(v ?? "free")}>
                <SelectTrigger className="min-h-11 w-full"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="free">Free</SelectItem>
                  <SelectItem value="starter">Starter</SelectItem>
                  <SelectItem value="pro">Pro</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="invite-days">Days valid</Label>
              <Input id="invite-days" type="number" min={1} value={days} onChange={(e) => setDays(Number(e.target.value))} className="min-h-11" />
            </div>
            <label className="flex min-h-11 items-center gap-3 text-sm">
              <input type="checkbox" checked={send} onChange={(e) => setSend(e.target.checked)} />
              Email the invitation
            </label>
            <Button
              className="min-h-11 w-full"
              disabled={pending || !canSubmit}
              onClick={async () => {
                setPending(true);
                const res = await adminInviteWaitlist(entry.id, { email: email.trim(), planId, days, send });
                setPending(false);
                if (!res.ok) { toast.error(messageForAdminError(res)); return; }
                setCreatedUrl(res.data.url);
                setEmailStatus(res.data.emailStatus);
                toast.success("Invitation created");
                onInvited({ ...entry, status: "invited", contactEmail: email.trim() || entry.contactEmail });
              }}
            >
              {pending ? "Creating..." : "Create invitation"}
            </Button>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
