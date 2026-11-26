"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { createClient } from "@/lib/api/clients";
import { messageForApiError } from "@/lib/api/errors";
import { useUnsavedChanges } from "@/components/dashboard/use-unsaved-changes";

export function NewClientForm() {
  const router = useRouter();
  const [name, setName] = useState("");
  const [phone, setPhone] = useState("");
  const [email, setEmail] = useState("");
  const [notes, setNotes] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const dirty = !!(name || phone || email || notes);
  useUnsavedChanges(dirty && !loading);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading(true);
    const res = await createClient({ name, phone: phone || undefined, email: email || undefined, notes: notes || undefined });
    setLoading(false);
    if (!res.ok) { setError(messageForApiError(res)); return; }
    toast.success("Client created");
    router.push(`/dashboard/clients/${res.data.id}`);
    router.refresh();
  }

  return (
    <div className="max-w-2xl space-y-5">
      <div><h1 className="text-2xl font-semibold tracking-tight">New client</h1><p className="text-sm text-muted-foreground">Add a client so you can attach shoots to them.</p></div>
      <Card><CardContent className="p-5">
        <form className="space-y-4" onSubmit={onSubmit}>
          <div className="space-y-2"><Label htmlFor="name">Name</Label><Input id="name" value={name} onChange={(e) => setName(e.target.value)} className="min-h-11" required /></div>
          <div className="space-y-2"><Label htmlFor="phone">Phone</Label><Input id="phone" value={phone} onChange={(e) => setPhone(e.target.value)} className="min-h-11" placeholder="0712 345 678" /></div>
          <div className="space-y-2"><Label htmlFor="email">Email</Label><Input id="email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} className="min-h-11" /></div>
          <div className="space-y-2"><Label htmlFor="notes">Notes</Label><Textarea id="notes" value={notes} onChange={(e) => setNotes(e.target.value)} /></div>
          {error ? <p className="text-sm text-destructive" role="status">{error}</p> : null}
          <div className="flex gap-3"><Button type="submit" className="min-h-11" disabled={loading}>{loading ? "Creating..." : "Create client"}</Button><Button type="button" variant="outline" className="min-h-11" onClick={() => router.push("/dashboard/clients")}>Cancel</Button></div>
        </form>
      </CardContent></Card>
    </div>
  );
}
