"use client";

import { useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { messageForAdminError } from "@/lib/admin/errors";
import { adminListAudit, type AuditListResponse } from "@/lib/api/admin";
import type { ApiError } from "@/lib/api/errors";

export function AuditScreen({ initial, error, initialAction }: { initial: AuditListResponse | null; error: ApiError | null; initialAction: string }) {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [data, setData] = useState(initial);
  const [err, setErr] = useState<ApiError | null>(error);
  const [action, setAction] = useState(initialAction);

  function load(next: { action?: string; offset?: number }) {
    const a = next.action ?? action; const offset = next.offset ?? 0;
    startTransition(async () => {
      const res = await adminListAudit({ action: a, limit: 25, offset });
      if (!res.ok) { setErr(res); return; }
      setErr(null); setData(res.data); setAction(a);
      const sp = new URLSearchParams(); if (a) sp.set("action", a); if (offset) sp.set("offset", String(offset));
      router.replace(`/admin/audit${sp.toString() ? `?${sp}` : ""}`);
    });
  }

  return (
    <div className="space-y-6">
      <div><h1 className="text-2xl font-semibold tracking-tight">Audit log</h1><p className="mt-1 text-sm text-muted-foreground">Newest first. State-changing admin actions are recorded here.</p></div>
      <div className="flex gap-2"><Input value={action} onChange={(e) => setAction(e.target.value)} placeholder="Filter by action" className="min-h-11 max-w-sm" /><Button variant="outline" className="min-h-11" disabled={pending} onClick={() => load({ action, offset: 0 })}>Filter</Button></div>
      {err ? <Card className="p-4 text-sm text-destructive">{messageForAdminError(err)} <Button variant="outline" className="ml-2 min-h-11" onClick={() => load({})}>Retry</Button></Card> : null}
      {!data || data.items.length === 0 ? <Card className="p-6 text-sm text-muted-foreground">No audit entries found.</Card> : (
        <>
          <Table><TableHeader><TableRow><TableHead>Time</TableHead><TableHead>Actor</TableHead><TableHead>Action</TableHead><TableHead>Target</TableHead><TableHead>IP</TableHead></TableRow></TableHeader>
            <TableBody>{data.items.map((e) => (
              <TableRow key={e.id}>
                <TableCell className="text-sm">{new Date(e.createdAt).toLocaleString()}</TableCell>
                <TableCell className="text-sm">{e.actor}</TableCell>
                <TableCell className="font-medium text-sm">{e.action}</TableCell>
                <TableCell className="text-sm text-muted-foreground">{e.targetType}{e.targetId ? `:${e.targetId.slice(0,8)}` : ""}</TableCell>
                <TableCell className="text-sm text-muted-foreground">{e.ip || "—"}</TableCell>
              </TableRow>
            ))}</TableBody>
          </Table>
          <div className="flex items-center gap-3"><Button variant="outline" className="min-h-11" disabled={pending || data.offset<=0} onClick={() => load({ offset: Math.max(0, data.offset-data.limit) })}>Previous</Button><Button variant="outline" className="min-h-11" disabled={pending || data.offset+data.limit>=data.total} onClick={() => load({ offset: data.offset+data.limit })}>Next</Button><p className="text-sm text-muted-foreground">{data.total} total</p></div>
        </>
      )}
    </div>
  );
}
