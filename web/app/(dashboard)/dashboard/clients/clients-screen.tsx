"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState, useTransition } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { listClients, type ClientListResponse } from "@/lib/api/clients";
import { messageForApiError, type ApiError } from "@/lib/api/errors";
import { formatKenyanPhone } from "@/lib/phone";

export function ClientsScreen({ initial, error, initialQuery }: { initial: ClientListResponse | null; error: ApiError | null; initialQuery: { q: string; offset: number } }) {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [q, setQ] = useState(initialQuery.q);
  const [data, setData] = useState(initial);
  const [err, setErr] = useState<ApiError | null>(error);

  function push(next: { q?: string; offset?: number }) {
    const nq = next.q ?? q;
    const off = next.offset ?? 0;
    const sp = new URLSearchParams();
    if (nq) sp.set("q", nq);
    if (off > 0) sp.set("offset", String(off));
    startTransition(async () => {
      const res = await listClients({ q: nq, limit: 25, offset: off });
      if (res.ok) { setData(res.data); setErr(null); } else setErr(res);
      router.replace(`/dashboard/clients${sp.toString() ? `?${sp}` : ""}`);
    });
  }

  const items = data?.items ?? [];
  return (
    <div className="space-y-5">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div><h1 className="text-2xl font-semibold tracking-tight">Clients</h1><p className="text-sm text-muted-foreground">Manage client details and their shoots.</p></div>
        <Button render={<Link href="/dashboard/clients/new" />} className="min-h-11">New client</Button>
      </div>
      <div className="flex flex-col gap-3 sm:flex-row">
        <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Search by name, phone, or email" className="min-h-11 sm:max-w-sm" aria-label="Search clients" />
        <Button variant="outline" className="min-h-11" onClick={() => push({ q, offset: 0 })} disabled={pending}>Search</Button>
      </div>
      {err ? <Card><CardContent className="p-5 flex items-center justify-between gap-3"><p className="text-sm text-destructive">{messageForApiError(err)}</p><Button variant="outline" className="min-h-11" onClick={() => push({})}>Retry</Button></CardContent></Card> : null}
      {!err && items.length === 0 ? <Card><CardContent className="p-6"><p className="text-sm text-muted-foreground">No clients found. Add a client to start creating shoots.</p></CardContent></Card> : null}
      {items.length > 0 ? (
        <>
          <div className="hidden md:block">
            <Table>
              <TableHeader><TableRow><TableHead>Name</TableHead><TableHead>Phone</TableHead><TableHead>Email</TableHead></TableRow></TableHeader>
              <TableBody>{items.map((c) => (
                <TableRow key={c.id}>
                  <TableCell><Link href={`/dashboard/clients/${c.id}`} className="font-medium hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-sm">{c.name}</Link></TableCell>
                  <TableCell>{c.phone ? formatKenyanPhone(c.phone) : "—"}</TableCell>
                  <TableCell>{c.email ?? "—"}</TableCell>
                </TableRow>
              ))}</TableBody>
            </Table>
          </div>
          <div className="space-y-3 md:hidden">{items.map((c) => (
            <Card key={c.id}><CardContent className="p-4 space-y-1"><Link href={`/dashboard/clients/${c.id}`} className="font-medium hover:underline">{c.name}</Link><p className="text-sm text-muted-foreground">{c.phone ? formatKenyanPhone(c.phone) : "—"}</p><p className="text-sm text-muted-foreground">{c.email ?? "—"}</p></CardContent></Card>
          ))}</div>
          <div className="flex items-center justify-between gap-3">
            <Button variant="outline" className="min-h-11" disabled={pending || initialQuery.offset <= 0} onClick={() => push({ offset: Math.max(0, initialQuery.offset - 25) })}>Previous</Button>
            <p className="text-sm text-muted-foreground">{data ? `${Math.min(data.offset + 1, data.total)}–${Math.min(data.offset + data.items.length, data.total)} of ${data.total}` : ""}</p>
            <Button variant="outline" className="min-h-11" disabled={pending || !data || initialQuery.offset + 25 >= data.total} onClick={() => push({ offset: initialQuery.offset + 25 })}>Next</Button>
          </div>
        </>
      ) : null}
    </div>
  );
}
