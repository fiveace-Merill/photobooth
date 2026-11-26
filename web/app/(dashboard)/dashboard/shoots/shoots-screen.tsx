"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState, useTransition } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { StatusChip } from "@/components/dashboard/status-chip";
import { OverdueLabel } from "@/components/dashboard/overdue-label";
import { listShoots, type ShootListResponse, type ShootStatus } from "@/lib/api/shoots";
import { messageForApiError, type ApiError } from "@/lib/api/errors";
import { formatMoney } from "@/lib/money";
import { formatDisplayDate } from "@/lib/dates";

const STATUSES: ShootStatus[] = ["shot", "editing", "ready", "delivered", "expired"];

export function ShootsScreen({
  initial, error, initialQuery,
}: {
  initial: ShootListResponse | null;
  error: ApiError | null;
  initialQuery: { q: string; status: string; overdue: boolean; offset: number };
}) {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [q, setQ] = useState(initialQuery.q);
  const [status, setStatus] = useState<string[]>(initialQuery.status ? initialQuery.status.split(",").filter(Boolean) : []);
  const [overdue, setOverdue] = useState(initialQuery.overdue);
  const [data, setData] = useState(initial);
  const [err, setErr] = useState<ApiError | null>(error);

  function push(next: { q?: string; status?: string[]; overdue?: boolean; offset?: number }) {
    const nq = next.q ?? q;
    const ns = next.status ?? status;
    const no = next.overdue ?? overdue;
    const off = next.offset ?? 0;
    const sp = new URLSearchParams();
    if (nq) sp.set("q", nq);
    if (ns.length) sp.set("status", ns.join(","));
    if (no) sp.set("overdue", "true");
    if (off > 0) sp.set("offset", String(off));
    startTransition(async () => {
      const res = await listShoots({ q: nq, status: ns.join(","), overdue: no ? true : undefined, limit: 25, offset: off, sort: "due" });
      if (res.ok) { setData(res.data); setErr(null); }
      else { setErr(res); }
      router.replace(`/dashboard/shoots${sp.toString() ? `?${sp}` : ""}`);
    });
  }

  function toggleStatus(s: ShootStatus) {
    const next = status.includes(s) ? status.filter((x) => x !== s) : [...status, s];
    setStatus(next);
    push({ status: next, offset: 0 });
  }

  const items = data?.items ?? [];

  return (
    <div className="space-y-5">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Shoots</h1>
          <p className="text-sm text-muted-foreground">Manage shoot progress, due dates, and delivery status.</p>
        </div>
        <Button render={<Link href="/dashboard/shoots/new" />} className="min-h-11">New shoot</Button>
      </div>

      <div className="flex flex-col gap-3">
        <div className="flex flex-col gap-3 sm:flex-row">
          <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Search by title" className="min-h-11 sm:max-w-xs" aria-label="Search shoots by title" />
          <Button variant="outline" className="min-h-11" onClick={() => push({ q, offset: 0 })} disabled={pending}>Search</Button>
        </div>
        <div className="flex flex-wrap gap-2" role="group" aria-label="Status filters">
          {STATUSES.map((s) => {
            const active = status.includes(s);
            return <Button key={s} variant={active ? "secondary" : "outline"} className="min-h-11" aria-pressed={active} onClick={() => toggleStatus(s)}>{s[0].toUpperCase() + s.slice(1)}</Button>;
          })}
          <Button variant={overdue ? "secondary" : "outline"} className="min-h-11" aria-pressed={overdue} onClick={() => { const n = !overdue; setOverdue(n); push({ overdue: n, offset: 0 }); }}>Overdue only</Button>
        </div>
      </div>

      {err ? <Card><CardContent className="p-5 flex items-center justify-between gap-3"><p className="text-sm text-destructive">{messageForApiError(err)}</p><Button variant="outline" className="min-h-11" onClick={() => push({})}>Retry</Button></CardContent></Card> : null}

      {!err && items.length === 0 ? (
        <Card><CardContent className="p-6"><p className="text-sm text-muted-foreground">No shoots found. Create a shoot to start tracking work.</p></CardContent></Card>
      ) : null}

      {items.length > 0 ? (
        <>
          <div className="hidden md:block">
            <Table>
              <TableHeader><TableRow><TableHead>Title</TableHead><TableHead>Client</TableHead><TableHead>Shoot date</TableHead><TableHead>Due date</TableHead><TableHead>Price</TableHead><TableHead>Status</TableHead></TableRow></TableHeader>
              <TableBody>
                {items.map((shoot) => (
                  <TableRow key={shoot.id}>
                    <TableCell><Link href={`/dashboard/shoots/${shoot.id}`} className="font-medium hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-sm">{shoot.title}</Link>{shoot.overdue ? <div className="mt-1"><OverdueLabel daysOverdue={shoot.daysOverdue} /></div> : null}</TableCell>
                    <TableCell>{shoot.clientName ?? "—"}</TableCell>
                    <TableCell>{shoot.shootDate ? formatDisplayDate(shoot.shootDate) : "—"}</TableCell>
                    <TableCell>{shoot.dueDate ? formatDisplayDate(shoot.dueDate) : "—"}</TableCell>
                    <TableCell>{formatMoney(shoot.priceMinor, shoot.currency)}</TableCell>
                    <TableCell><StatusChip status={shoot.status} /></TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          <div className="space-y-3 md:hidden">
            {items.map((shoot) => (
              <Card key={shoot.id}><CardContent className="p-4 space-y-2">
                <div className="flex items-start justify-between gap-3"><Link href={`/dashboard/shoots/${shoot.id}`} className="font-medium hover:underline">{shoot.title}</Link><StatusChip status={shoot.status} /></div>
                <p className="text-sm text-muted-foreground">{shoot.clientName ?? "—"}</p>
                <p className="text-sm text-muted-foreground">Shoot: {shoot.shootDate ? formatDisplayDate(shoot.shootDate) : "—"}</p>
                <p className="text-sm text-muted-foreground">Due: {shoot.dueDate ? formatDisplayDate(shoot.dueDate) : "—"}</p>
                <p className="text-sm text-muted-foreground">{formatMoney(shoot.priceMinor, shoot.currency)}</p>
                {shoot.overdue ? <OverdueLabel daysOverdue={shoot.daysOverdue} /> : null}
              </CardContent></Card>
            ))}
          </div>
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
