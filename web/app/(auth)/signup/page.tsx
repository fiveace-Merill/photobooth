"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { signup } from "@/lib/auth/client";

export default function SignupPage() {
  const router = useRouter();
  const [form, setForm] = useState({
    email: "",
    password: "",
    business_name: "",
    slug: "",
  });
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  function set<K extends keyof typeof form>(key: K, value: string) {
    setForm((f) => ({ ...f, [key]: value }));
  }

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading(true);
    const result = await signup(form);
    setLoading(false);
    if (!result.ok) {
      setError(result.error);
      return;
    }
    router.push("/dashboard");
    router.refresh();
  }

  return (
    <main className="mx-auto flex min-h-screen max-w-md flex-col justify-center px-6 py-16">
      <p className="text-sm font-medium text-muted-foreground">Photobooth</p>
      <h1 className="mt-3 text-4xl font-semibold tracking-tight">Create your account</h1>
      <p className="mt-3 text-muted-foreground">
        Start delivering photos to your clients.
      </p>

      <form className="mt-8 space-y-4" onSubmit={onSubmit}>
        <Field id="business_name" label="Business name" value={form.business_name} onChange={(v) => set("business_name", v)} autoComplete="organization" />
        <Field id="slug" label="Studio slug" value={form.slug} onChange={(v) => set("slug", v)} autoComplete="off" hint="Used in your public link, e.g. studio-one" />
        <Field id="email" label="Email" type="email" value={form.email} onChange={(v) => set("email", v)} autoComplete="email" />
        <Field id="password" label="Password" type="password" value={form.password} onChange={(v) => set("password", v)} autoComplete="new-password" hint="At least 8 characters" />
        {error ? <p className="text-sm text-destructive" role="alert">{error}</p> : null}
        <Button type="submit" disabled={loading} className="min-h-11 w-full">
          {loading ? "Creating account..." : "Create account"}
        </Button>
      </form>

      <p className="mt-6 text-sm text-muted-foreground">
        Already have an account?{" "}
        <Link href="/login" className="font-medium text-foreground underline underline-offset-4">
          Log in
        </Link>
      </p>
    </main>
  );
}

function Field(props: {
  id: string;
  label: string;
  value: string;
  onChange: (v: string) => void;
  type?: string;
  autoComplete?: string;
  hint?: string;
}) {
  return (
    <div className="space-y-2">
      <label className="text-sm font-medium" htmlFor={props.id}>{props.label}</label>
      <Input
        id={props.id}
        type={props.type ?? "text"}
        autoComplete={props.autoComplete}
        required
        value={props.value}
        onChange={(e) => props.onChange(e.target.value)}
        className="min-h-11"
      />
      {props.hint ? <p className="text-xs text-muted-foreground">{props.hint}</p> : null}
    </div>
  );
}
