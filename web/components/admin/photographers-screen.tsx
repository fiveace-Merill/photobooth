"use client";

import { useState, useTransition } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { AdminStatusChip } from "./status-chip";
import { photographerStatusChip } from "@/lib/admin/status";
import { messageForAdminError } from "@/lib/admin/errors";
import { adminListPhotographers, type AdminPhotographersResponse } from "@/lib/api/admin";
import type { ApiError } from "@/lib/api/errors";

export function PhotographersScreen({ initial, error, initialQuery, initialStatus }: { initial: AdminPhotographersResponse | null; error: ApiError | null; initialQuery: string; initialStatus: string; }) {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [data, setData] = useState(initial);
  const [err, setErr] = useState<ApiError | null>(error);
  const [q, setQ] = useState(initialQuery);
  const [status, setStatus] = useState(initialStatus);

  function load(next: { q?: string; status?: string; offset?: number }) {
    const nq = next.q ?? q; const ns = next.status ?? status; const offset = next.offset ?? 0;
    startTransition(async () => {
      const res = await adminListPhotographers({ q: nq, status: ns, limit: 20, offset });
      if (!res.ok) { setErr(res); return; }
      setErr(null); setData(res.data); setQ(nq); setStatus(ns);
      const sp = new URLSearchParams();
      if (nq) sp.set("q", nq); if (ns) sp.set("status", ns); if (offset) sp.set("offset", String(offset));
      router.replace(`/admin/photographers${sp.toString() ? `?${sp}` : ""}`);
    });
  }

  return (
    <div className="space-y-6">
      <div><h1 className="text-2xl font-semibold tracking-tight">Photographers</h1><p className="mt-1 text-sm text-muted-foreground">Platform accounts and plan usage. No client or shoot content is shown here.</p></div>
      <div className="flex flex-wrap gap-2">
        {["", "active", "suspended"].map((s) => <Button key={s || "all"} variant={status===s?"default":"outline"} className="min-h-11" disabled={pending} onClick={() => load({ status: s, offset: 0 })}>{s ? s[0].toUpperCase()+s.slice(1) : "All"}</Button>)}
      </div>
      <div className="flex gap-2"><Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Search email, business, slug" className="min-h-11 max-w-sm" /><Button variant="outline" className="min-h-11" disabled={pending} onClick={() => load({ q, offset: 0 })}>Search</Button></div>
      {err ? <Card className="p-4 text-sm text-destructive">{messageForAdminError(err)} <Button variant="outline" className="ml-2 min-h-11" onClick={() => load({})}>Retry</Button></Card> : null}
      {!data || data.items.length === 0 ? <Card className="p-6 text-sm text-muted-foreground">No photographers found.</Card> : (
        <>
          <div className="hidden md:block">
            <Table><TableHeader><TableRow><TableHead>Business</TableHead><TableHead>Plan</TableHead><TableHead>Status</TableHead><TableHead>Storage</TableHead><TableHead>Usage</TableHead><TableHead>Last login</TableHead></TableRow></TableHeader>
              <TableBody>{data.items.map((p) => (
                <TableRow key={p.id}>
                  <TableCell><Link className="font-medium underline-offset-4 hover:underline" href={`/admin/photographers/${p.id}`}>{p.businessName}</Link><div className="text-xs text-muted-foreground">{p.email}</div></TableCell>
                  <TableCell>{p.planId}</TableCell>
                  <TableCell><AdminStatusChip chip={photographerStatusChip(p.status)} /></TableCell>
                  <TableCell><div className="text-sm">{formatBytes(p.storageUsedBytes)} / {formatBytes(p.storageQuotaBytes)}</div><div className="mt-1 h-2 w-32 rounded bg-muted"><div className="h-2 rounded bg-foreground" style={{ width: `${quotaPct(p.storageUsedBytes, p.storageQuotaBytes)}%` }} /></div></TableCell>
                  <TableCell className="text-sm text-muted-foreground">{p.shoots} shoots · {p.photos} photos · {p.delivered} delivered</TableCell>
                  <TableCell className="text-sm text-muted-foreground">{p.lastLoginAt ? new Date(p.lastLoginAt).toLocaleDateString() : "Never"}</TableCell>
                </TableRow>
              ))}</TableBody>
            </Table>
          </div>
          <div className="space-y-3 md:hidden">{data.items.map((p) => (
            <Link key={p.id} href={`/admin/photographers/${p.id}`} className="block rounded-xl border border-border bg-card p-4">
              <div className="flex items-center justify-between gap-3"><p className="font-medium">{p.businessName}</p><AdminStatusChip chip={photographerStatusChip(p.status)} /></div>
              <p className="text-sm text-muted-foreground">{p.email}</p>
              <p className="mt-2 text-sm">{formatBytes(p.storageUsedBytes)} / {formatBytes(p.storageQuotaBytes)}</p>
              <div className="mt-1 h-2 w-full rounded bg-muted"><div className="h-2 rounded bg-foreground" style={{ width: `${quotaPct(p.storageUsedBytes, p.storageQuotaBytes)}%` }} /></div>
            </Link>
          ))}</div>
          <div className="flex items-center gap-3"><Button variant="outline" className="min-h-11" disabled={pending || data.offset<=0} onClick={() => load({ offset: Math.max(0, data.offset-data.limit) })}>Previous</Button><Button variant="outline" className="min-h-11" disabled={pending || data.offset+data.limit>=data.total} onClick={() => load({ offset: data.offset+data.limit })}>Next</Button><p className="text-sm text-muted-foreground">{data.total} total</p></div>
        </>
      )}
    </div>
  );
}

function formatBytes(bytes: number): string { if (!bytes) return "0 B"; const u=["B","KB","MB","GB","TB"]; let i=0,v=bytes; while(v>=1024&&i<u.length-1){v/=1024;i++;} return `${v>=10||i===0?v.toFixed(0):v.toFixed(1)} ${u[i]}`; }
function quotaPct(used: number, quota: number): number { if (!quota) return 0; return Math.min(100, Math.round((used/quota)*100)); }
