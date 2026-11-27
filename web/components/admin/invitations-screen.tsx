"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { AdminStatusChip } from "./status-chip";
import { invitationStatusChip } from "@/lib/admin/status";
import { messageForAdminError } from "@/lib/admin/errors";
import { adminResendInvitation, adminRevokeInvitation, type Invitation } from "@/lib/api/admin";
import type { ApiError } from "@/lib/api/errors";

export function InvitationsScreen({ initial, error }: { initial: { items: Invitation[] } | null; error: ApiError | null }) {
  const router = useRouter();
  const [items, setItems] = useState(initial?.items ?? []);
  const [err] = useState<ApiError | null>(error);
  const [resend, setResend] = useState<Invitation | null>(null);

  if (err) return <Card className="p-4 text-sm text-destructive">{messageForAdminError(err)}</Card>;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Invitations</h1>
        <p className="mt-1 text-sm text-muted-foreground">Track and manage photographer invitations.</p>
      </div>
      {items.length === 0 ? <Card className="p-6 text-sm text-muted-foreground">No invitations yet.</Card> : (
        <Table>
          <TableHeader><TableRow><TableHead>Email</TableHead><TableHead>Plan</TableHead><TableHead>Status</TableHead><TableHead>Expires</TableHead><TableHead>Actions</TableHead></TableRow></TableHeader>
          <TableBody>
            {items.map((inv) => (
              <TableRow key={inv.id}>
                <TableCell className="font-medium">{inv.email}</TableCell>
                <TableCell>{inv.planId}</TableCell>
                <TableCell><AdminStatusChip chip={invitationStatusChip(inv.status)} /></TableCell>
                <TableCell>{new Date(inv.expiresAt).toLocaleDateString()}</TableCell>
                <TableCell>
                  <div className="flex flex-wrap gap-2">
                    <Button variant="outline" className="min-h-11" onClick={() => setResend(inv)}>Resend</Button>
                    <Button variant="outline" className="min-h-11" disabled={inv.status !== "pending"} onClick={async () => {
                      const res = await adminRevokeInvitation(inv.id);
                      if (!res.ok) { toast.error(messageForAdminError(res)); return; }
                      toast.success("Invitation revoked");
                      setItems((xs) => xs.map((x) => x.id === inv.id ? { ...x, status: "revoked" } : x));
                      router.refresh();
                    }}>Revoke</Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
      <ResendDialog invitation={resend} onClose={() => setResend(null)} onResent={(inv) => { setItems((xs) => [inv, ...xs.map(x => x.id === resend?.id ? { ...x, status: "revoked" } : x)]); router.refresh(); }} />
    </div>
  );
}

function ResendDialog({ invitation, onClose, onResent }: { invitation: Invitation | null; onClose: () => void; onResent: (inv: Invitation) => void }) {
  const [days, setDays] = useState(7);
  const [send, setSend] = useState(false);
  const [pending, setPending] = useState(false);
  const [url, setUrl] = useState<string | null>(null);
  const [emailStatus, setEmailStatus] = useState<string | null>(null);
  return (
    <Dialog open={!!invitation} onOpenChange={(v) => { if (!v) { onClose(); setUrl(null); setEmailStatus(null); } }}>
      <DialogContent>
        <DialogHeader><DialogTitle>Resend invitation</DialogTitle></DialogHeader>
        {url ? (
          <div className="space-y-4">
            <p className="text-sm text-muted-foreground">The old invitation link stops working. Share this new link now; it cannot be shown again.</p>
            <Input readOnly value={url} className="min-h-11 font-mono text-xs" />
            <div className="flex gap-2"><Button variant="outline" className="min-h-11" onClick={() => navigator.clipboard.writeText(url)}>Copy link</Button></div>
            {emailStatus ? <p className="text-sm text-muted-foreground">Email status: {emailStatus}</p> : null}
            <p className="text-sm text-destructive">This link is shown once.</p>
          </div>
        ) : (
          <div className="space-y-4">
            <p className="text-sm text-muted-foreground">Resending revokes the old link and creates a new one.</p>
            <div className="space-y-2"><Label htmlFor="resend-days">Days valid</Label><Input id="resend-days" type="number" min={1} value={days} onChange={(e) => setDays(Number(e.target.value))} className="min-h-11" /></div>
            <label className="flex min-h-11 items-center gap-3 text-sm"><input type="checkbox" checked={send} onChange={(e) => setSend(e.target.checked)} />Email the invitation</label>
            <Button className="min-h-11 w-full" disabled={pending || !invitation} onClick={async () => {
              if (!invitation) return;
              setPending(true);
              const res = await adminResendInvitation(invitation.id, { days, send });
              setPending(false);
              if (!res.ok) { toast.error(messageForAdminError(res)); return; }
              setUrl(res.data.url);
              setEmailStatus(res.data.emailStatus);
              if (res.data.invitation) onResent(res.data.invitation);
              toast.success("Invitation resent");
            }}>{pending ? "Resending..." : "Resend invitation"}</Button>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
