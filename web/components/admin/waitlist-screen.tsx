"use client";

import { useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { AdminStatusChip } from "./status-chip";
import { InviteDialog } from "./invite-dialog";
import { messageForAdminError } from "@/lib/admin/errors";
import { waitlistStatusChip } from "@/lib/admin/status";
import { csvFilename, downloadCsvResponse } from "@/lib/admin/csv";
import { isTypedConfirmationMatch } from "@/lib/admin/confirm";
import {
  adminDeclineWaitlist,
  adminDeleteWaitlist,
  adminExportWaitlistCsv,
  adminListWaitlist,
  adminPatchWaitlist,
  adminSpamWaitlist,
  type WaitlistEntry,
  type WaitlistListResponse,
} from "@/lib/api/admin";
import type { ApiError } from "@/lib/api/errors";

const statuses = ["", "new", "invited", "declined", "spam"] as const;

export function WaitlistScreen({
  initial,
  error,
  initialStatus,
  initialQuery,
}: {
  initial: WaitlistListResponse | null;
  error: ApiError | null;
  initialStatus: string;
  initialQuery: string;
}) {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [data, setData] = useState(initial);
  const [err, setErr] = useState<ApiError | null>(error);
  const [status, setStatus] = useState(initialStatus);
  const [query, setQuery] = useState(initialQuery);
  const [selected, setSelected] = useState<WaitlistEntry | null>(null);

  function load(next: { status?: string; q?: string; offset?: number }) {
    const s = next.status ?? status;
    const q = next.q ?? query;
    const offset = next.offset ?? 0;
    startTransition(async () => {
      const res = await adminListWaitlist({ status: s, q, limit: 20, offset });
      if (!res.ok) { setErr(res); return; }
      setErr(null);
      setData(res.data);
      setStatus(s);
      setQuery(q);
      const params = new URLSearchParams();
      if (s) params.set("status", s);
      if (q) params.set("q", q);
      if (offset) params.set("offset", String(offset));
      router.replace(`/admin/waitlist${params.toString() ? `?${params}` : ""}`);
    });
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Waitlist</h1>
          <p className="mt-1 text-sm text-muted-foreground">Review early-access requests and invite photographers.</p>
        </div>
        <ExportCsvButton />
      </div>

      <div className="flex flex-wrap gap-2">
        {statuses.map((s) => (
          <Button key={s || "all"} variant={status === s ? "default" : "outline"} className="min-h-11" onClick={() => load({ status: s, offset: 0 })} disabled={pending}>
            {s ? s[0].toUpperCase() + s.slice(1) : "All"}
          </Button>
        ))}
      </div>

      <div className="flex gap-2">
        <Input value={query} onChange={(e) => setQuery(e.target.value)} placeholder="Search name, email, phone" className="min-h-11 max-w-sm" />
        <Button className="min-h-11" variant="outline" onClick={() => load({ q: query, offset: 0 })} disabled={pending}>Search</Button>
      </div>

      {err ? (
        <Card className="p-4 text-sm text-destructive">
          <p>{messageForAdminError(err)}</p>
          <Button className="mt-3 min-h-11" variant="outline" onClick={() => load({})}>Retry</Button>
        </Card>
      ) : null}

      {!data || data.items.length === 0 ? (
        <Card className="p-6 text-sm text-muted-foreground">No waitlist entries found.</Card>
      ) : (
        <>
          <div className="hidden md:block">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead><TableHead>Contact</TableHead><TableHead>Type</TableHead><TableHead>Status</TableHead><TableHead>Created</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.items.map((e) => (
                  <TableRow key={e.id} className="cursor-pointer" onClick={() => setSelected(e)}>
                    <TableCell className="font-medium">{e.name}</TableCell>
                    <TableCell>{e.contactEmail || e.contactPhone || "—"}</TableCell>
                    <TableCell>{e.photographyType}</TableCell>
                    <TableCell><AdminStatusChip chip={waitlistStatusChip(e.status)} /></TableCell>
                    <TableCell>{new Date(e.createdAt).toLocaleDateString()}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          <div className="space-y-3 md:hidden">
            {data.items.map((e) => (
              <button key={e.id} className="w-full rounded-xl border border-border bg-card p-4 text-left" onClick={() => setSelected(e)}>
                <div className="flex items-center justify-between gap-3">
                  <p className="font-medium">{e.name}</p>
                  <AdminStatusChip chip={waitlistStatusChip(e.status)} />
                </div>
                <p className="mt-1 text-sm text-muted-foreground">{e.contactEmail || e.contactPhone || "—"}</p>
                <p className="mt-1 text-sm text-muted-foreground">{e.photographyType}</p>
              </button>
            ))}
          </div>
          <div className="flex items-center gap-3">
            <Button variant="outline" className="min-h-11" disabled={pending || data.offset <= 0} onClick={() => load({ offset: Math.max(0, data.offset - data.limit) })}>Previous</Button>
            <Button variant="outline" className="min-h-11" disabled={pending || data.offset + data.limit >= data.total} onClick={() => load({ offset: data.offset + data.limit })}>Next</Button>
            <p className="text-sm text-muted-foreground">{data.total} total</p>
          </div>
        </>
      )}

      <WaitlistPanel
        entry={selected}
        onClose={() => setSelected(null)}
        onChanged={(next) => {
          setSelected(next);
          setData((d) => d ? { ...d, items: d.items.map((i) => i.id === next.id ? next : i) } : d);
          router.refresh();
        }}
        onDeleted={(id) => {
          setSelected(null);
          setData((d) => d ? { ...d, items: d.items.filter((i) => i.id !== id), total: Math.max(0, d.total - 1) } : d);
          router.refresh();
        }}
      />
    </div>
  );
}

function ExportCsvButton() {
  const [pending, setPending] = useState(false);
  return (
    <div className="space-y-1">
      <Button
        variant="outline"
        className="min-h-11"
        disabled={pending}
        onClick={async () => {
          setPending(true);
          const res = await adminExportWaitlistCsv();
          setPending(false);
          if (!res.ok) { toast.error(messageForAdminError(res)); return; }
          await downloadCsvResponse(res.data, csvFilename("waitlist"));
          toast.success("Waitlist CSV exported");
        }}
      >
        {pending ? "Exporting..." : "Export CSV"}
      </Button>
      <p className="text-xs text-muted-foreground">Contains personal data. Export is logged.</p>
    </div>
  );
}

function WaitlistPanel({ entry, onClose, onChanged, onDeleted }: { entry: WaitlistEntry | null; onClose: () => void; onChanged: (e: WaitlistEntry) => void; onDeleted: (id: string) => void; }) {
  const [notes, setNotes] = useState("");
  const [saving, setSaving] = useState(false);
  const [deleteText, setDeleteText] = useState("");
  const open = !!entry;
  const current = entry;

  return (
    <Sheet open={open} onOpenChange={(v) => !v && onClose()}>
      <SheetContent side="right" className="w-[min(34rem,100vw)] overflow-y-auto sm:max-w-xl">
        {current ? (
          <div className="space-y-5 p-4">
            <SheetHeader className="p-0">
              <SheetTitle>{current.name}</SheetTitle>
            </SheetHeader>
            <div className="flex items-center gap-2"><AdminStatusChip chip={waitlistStatusChip(current.status)} /></div>
            <dl className="grid gap-3 text-sm">
              <Field label="Email" value={current.contactEmail || "—"} />
              <Field label="Phone" value={current.contactPhone || "—"} />
              <Field label="Photography type" value={current.photographyType} />
              <Field label="Shoots per month" value={current.shootsPerMonth} />
              <Field label="Consent" value={`${new Date(current.consentAt).toLocaleString()} (${current.consentVersion})`} />
              <Field label="Created" value={new Date(current.createdAt).toLocaleString()} />
            </dl>

            <div className="space-y-2">
              <label className="text-sm font-medium" htmlFor="waitlist-notes">Notes</label>
              <textarea id="waitlist-notes" className="min-h-24 w-full rounded-md border border-input bg-background px-3 py-2 text-sm" defaultValue={current.notes} onChange={(e) => setNotes(e.target.value)} />
              <Button variant="outline" className="min-h-11" disabled={saving} onClick={async () => {
                setSaving(true);
                const res = await adminPatchWaitlist(current.id, { notes });
                setSaving(false);
                if (!res.ok) { toast.error(messageForAdminError(res)); return; }
                toast.success("Notes saved");
                onChanged(res.data);
              }}>Save notes</Button>
            </div>

            <div className="flex flex-wrap gap-2">
              <InviteDialog entry={current} onInvited={onChanged} />
              <ActionButton label="Decline" onClick={async () => {
                const res = await adminDeclineWaitlist(current.id);
                if (!res.ok) { toast.error(messageForAdminError(res)); return; }
                toast.success("Waitlist entry declined");
                onChanged(res.data);
              }} />
              <ActionButton label="Mark as spam" onClick={async () => {
                const res = await adminSpamWaitlist(current.id);
                if (!res.ok) { toast.error(messageForAdminError(res)); return; }
                toast.success("Waitlist entry marked as spam");
                onChanged(res.data);
              }} />
            </div>

            <div className="rounded-lg border border-destructive/30 p-4 space-y-3">
              <p className="text-sm font-medium text-destructive">Delete entry</p>
              <p className="text-sm text-muted-foreground">This permanently deletes the waitlist entry. This is used for erasure requests and cannot be undone.</p>
              <Input value={deleteText} onChange={(e) => setDeleteText(e.target.value)} placeholder='Type DELETE to confirm' className="min-h-11" />
              <Button variant="destructive" className="min-h-11" disabled={!isTypedConfirmationMatch(deleteText, "DELETE")} onClick={async () => {
                const res = await adminDeleteWaitlist(current.id);
                if (!res.ok) { toast.error(messageForAdminError(res)); return; }
                toast.success("Waitlist entry deleted");
                onDeleted(current.id);
              }}>Delete permanently</Button>
            </div>
          </div>
        ) : null}
      </SheetContent>
    </Sheet>
  );
}

function ActionButton({ label, onClick }: { label: string; onClick: () => Promise<void> }) {
  const [pending, setPending] = useState(false);
  return <Button variant="outline" className="min-h-11" disabled={pending} onClick={async () => { setPending(true); await onClick(); setPending(false); }}>{label}</Button>;
}

function Field({ label, value }: { label: string; value: string }) {
  return <div><dt className="text-muted-foreground">{label}</dt><dd className="font-medium break-words">{value}</dd></div>;
}
