"use client";

import { useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Command, CommandEmpty, CommandInput, CommandItem, CommandList } from "@/components/ui/command";
import { createClient, type Client } from "@/lib/api/clients";
import { createShoot } from "@/lib/api/shoots";
import { messageForApiError } from "@/lib/api/errors";
import { parseKesToMinor } from "@/lib/money";
import { formatKenyanPhone } from "@/lib/phone";
import { useUnsavedChanges } from "@/components/dashboard/use-unsaved-changes";

export function NewShootForm({ initialClients }: { initialClients: Client[] }) {
  const router = useRouter();
  const [clients, setClients] = useState(initialClients);
  const [clientOpen, setClientOpen] = useState(false);
  const [clientQuery, setClientQuery] = useState("");
  const [clientId, setClientId] = useState("");
  const [title, setTitle] = useState("");
  const [shootDate, setShootDate] = useState("");
  const [dueDate, setDueDate] = useState("");
  const [price, setPrice] = useState("0");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const [addOpen, setAddOpen] = useState(false);
  const [newName, setNewName] = useState("");
  const [newPhone, setNewPhone] = useState("");
  const [newEmail, setNewEmail] = useState("");
  const [newClientError, setNewClientError] = useState<string | null>(null);
  const [newClientLoading, setNewClientLoading] = useState(false);

  const dirty = !!(clientId || title || shootDate || dueDate || price !== "0");
  useUnsavedChanges(dirty && !loading);

  const filtered = useMemo(() => {
    const q = clientQuery.toLowerCase().trim();
    if (!q) return clients;
    return clients.filter((c) => [c.name, c.phone, c.email].filter(Boolean).some((v) => String(v).toLowerCase().includes(q)));
  }, [clients, clientQuery]);

  const selected = clients.find((c) => c.id === clientId);

  async function onAddClient() {
    setNewClientError(null);
    setNewClientLoading(true);
    const res = await createClient({ name: newName, phone: newPhone || undefined, email: newEmail || undefined });
    setNewClientLoading(false);
    if (!res.ok) { setNewClientError(messageForApiError(res)); return; }
    setClients((prev) => [res.data, ...prev]);
    setClientId(res.data.id);
    setAddOpen(false);
    setNewName(""); setNewPhone(""); setNewEmail("");
    toast.success("Client created");
  }

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    if (!clientId) { setError("Select a client."); return; }
    let priceMinor = 0;
    try { priceMinor = parseKesToMinor(price); } catch { setError("Enter a valid whole KES price, like 3,000."); return; }
    setLoading(true);
    const res = await createShoot({ clientId, title, shootDate: shootDate || undefined, dueDate: dueDate || undefined, priceMinor, currency: "KES" });
    setLoading(false);
    if (!res.ok) { setError(messageForApiError(res)); return; }
    toast.success("Shoot created");
    router.push(`/dashboard/shoots/${res.data.id}`);
    router.refresh();
  }

  return (
    <div className="max-w-2xl space-y-5">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">New shoot</h1>
        <p className="text-sm text-muted-foreground">Create a shoot and track it through delivery.</p>
      </div>
      <Card><CardContent className="p-5">
        <form className="space-y-5" onSubmit={onSubmit}>
          <div className="space-y-2">
            <Label>Client</Label>
            <div className="flex flex-col gap-2 sm:flex-row">
              <Popover open={clientOpen} onOpenChange={setClientOpen}>
                <PopoverTrigger render={<Button type="button" variant="outline" className="min-h-11 justify-start sm:flex-1" />}>
                  {selected ? `${selected.name}${selected.phone ? ` (${formatKenyanPhone(selected.phone)})` : ""}` : "Select client"}
                </PopoverTrigger>
                <PopoverContent className="w-[min(24rem,calc(100vw-2rem))] p-0">
                  <Command>
                    <CommandInput value={clientQuery} onChange={(e) => setClientQuery(e.target.value)} placeholder="Search clients..." aria-label="Search clients" />
                    <CommandList>
                      {filtered.length === 0 ? <CommandEmpty>No clients found.</CommandEmpty> : null}
                      {filtered.map((c) => (
                        <CommandItem key={c.id} onClick={() => { setClientId(c.id); setClientOpen(false); }}>
                          <div><div className="font-medium">{c.name}</div><div className="text-xs text-muted-foreground">{c.phone ? formatKenyanPhone(c.phone) : c.email}</div></div>
                        </CommandItem>
                      ))}
                    </CommandList>
                  </Command>
                </PopoverContent>
              </Popover>
              <Dialog open={addOpen} onOpenChange={setAddOpen}>
                <DialogTrigger render={<Button type="button" variant="outline" className="min-h-11" />}>Add new client</DialogTrigger>
                <DialogContent>
                  <DialogHeader><DialogTitle>Add client</DialogTitle><DialogDescription>Create a client without leaving this form.</DialogDescription></DialogHeader>
                  <div className="space-y-3">
                    <div className="space-y-2"><Label htmlFor="new-client-name">Name</Label><Input id="new-client-name" value={newName} onChange={(e) => setNewName(e.target.value)} className="min-h-11" /></div>
                    <div className="space-y-2"><Label htmlFor="new-client-phone">Phone</Label><Input id="new-client-phone" value={newPhone} onChange={(e) => setNewPhone(e.target.value)} className="min-h-11" placeholder="0712 345 678" /></div>
                    <div className="space-y-2"><Label htmlFor="new-client-email">Email</Label><Input id="new-client-email" type="email" value={newEmail} onChange={(e) => setNewEmail(e.target.value)} className="min-h-11" /></div>
                    {newClientError ? <p className="text-sm text-destructive" role="status">{newClientError}</p> : null}
                  </div>
                  <DialogFooter>
                    <Button type="button" variant="outline" className="min-h-11" onClick={() => setAddOpen(false)}>Cancel</Button>
                    <Button type="button" className="min-h-11" onClick={onAddClient} disabled={newClientLoading}>{newClientLoading ? "Creating..." : "Create client"}</Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
            </div>
          </div>
          <div className="space-y-2"><Label htmlFor="title">Title</Label><Input id="title" value={title} onChange={(e) => setTitle(e.target.value)} className="min-h-11" required /></div>
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2"><Label htmlFor="shootDate">Shoot date</Label><Input id="shootDate" type="date" value={shootDate} onChange={(e) => setShootDate(e.target.value)} className="min-h-11" /></div>
            <div className="space-y-2"><Label htmlFor="dueDate">Due date</Label><Input id="dueDate" type="date" value={dueDate} onChange={(e) => setDueDate(e.target.value)} className="min-h-11" /></div>
          </div>
          <div className="space-y-2">
            <Label htmlFor="price">Price (KES)</Label>
            <Input id="price" inputMode="numeric" value={price} onChange={(e) => setPrice(e.target.value)} className="min-h-11" />
            <p className="text-xs text-muted-foreground">Enter whole KES, for example 3,000. Use 0 for free delivery.</p>
          </div>
          {error ? <p className="text-sm text-destructive" role="status">{error}</p> : null}
          <div className="flex gap-3">
            <Button type="submit" className="min-h-11" disabled={loading}>{loading ? "Creating..." : "Create shoot"}</Button>
            <Button type="button" variant="outline" className="min-h-11" onClick={() => router.push("/dashboard/shoots")}>Cancel</Button>
          </div>
        </form>
      </CardContent></Card>
    </div>
  );
}
