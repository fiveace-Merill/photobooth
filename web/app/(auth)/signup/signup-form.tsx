"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { authConfig, getInvitation, me, signup } from "@/lib/auth/client";
import { site } from "@/lib/site";
import {
  mapAuthError,
  suggestSlug,
  validateEmail,
  validatePassword,
  validateSlug,
} from "@/lib/validation/auth";

type FieldErrors = Partial<Record<"business_name" | "email" | "password" | "slug", string>>;
type Mode =
  | { kind: "loading" }
  | { kind: "open" }
  | { kind: "invite_only" }
  | { kind: "invite_invalid" }
  | { kind: "invite_valid"; email: string; token: string };

export function SignupForm({ inviteToken }: { inviteToken?: string }) {
  const router = useRouter();
  const [mode, setMode] = useState<Mode>({ kind: inviteToken ? "loading" : "loading" });
  const [form, setForm] = useState({ business_name: "", email: "", password: "", slug: "" });
  const [slugEdited, setSlugEdited] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [errors, setErrors] = useState<FieldErrors>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    me().then((result) => {
      if (result.ok) {
        router.replace("/dashboard");
        router.refresh();
      }
    });
  }, [router]);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      if (inviteToken) {
        const res = await getInvitation(inviteToken);
        if (cancelled) return;
        if (res.ok) {
          setMode({ kind: "invite_valid", email: res.invitation.email, token: inviteToken });
          setForm((f) => ({ ...f, email: res.invitation.email }));
          return;
        }
        setMode({ kind: "invite_invalid" });
        return;
      }
      const cfg = await authConfig();
      if (cancelled) return;
      if (cfg.ok && cfg.config.signupMode === "open") {
        setMode({ kind: "open" });
        return;
      }
      setMode({ kind: "invite_only" });
    }
    void load();
    return () => {
      cancelled = true;
    };
  }, [inviteToken]);

  function set<K extends keyof typeof form>(key: K, value: string) {
    setForm((f) => ({ ...f, [key]: value }));
    setErrors((e) => ({ ...e, [key]: undefined }));
  }

  function onBusinessNameChange(value: string) {
    set("business_name", value);
    if (!slugEdited) {
      setForm((f) => ({ ...f, business_name: value, slug: suggestSlug(value) }));
    }
  }

  function onSlugChange(value: string) {
    setSlugEdited(true);
    set("slug", value.toLowerCase().replace(/[^a-z0-9-]/g, ""));
  }

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setFormError(null);
    const nextErrors: FieldErrors = {};
    if (!form.business_name.trim()) nextErrors.business_name = "Enter your business name.";
    const emailV = validateEmail(form.email);
    if (!emailV.ok) nextErrors.email = emailV.message;
    const passV = validatePassword(form.password);
    if (!passV.ok) nextErrors.password = passV.message;
    const slugV = validateSlug(form.slug);
    if (!slugV.ok) nextErrors.slug = slugV.message;
    setErrors(nextErrors);
    const first = (["business_name", "email", "password", "slug"] as const).find((k) => nextErrors[k]);
    if (first) {
      document.getElementById(first)?.focus();
      return;
    }

    setLoading(true);
    const result = await signup({
      email: form.email.trim(),
      password: form.password,
      business_name: form.business_name.trim(),
      slug: form.slug.trim(),
      inviteToken: mode.kind === "invite_valid" ? mode.token : undefined,
    });
    setLoading(false);
    if (!result.ok) {
      const mapped = mapAuthError(result);
      if (mapped === "signup_closed") {
        setMode({ kind: "invite_only" });
        return;
      }
      if (mapped === "invite_invalid") {
        setMode({ kind: "invite_invalid" });
        return;
      }
      if (result.code === "email_taken") setErrors({ email: mapped });
      else if (result.code === "slug_taken") setErrors({ slug: mapped });
      else setFormError(mapped);
      return;
    }

    if (mode.kind === "invite_valid") {
      // Remove invite token from the address bar after a successful signup.
      window.history.replaceState(null, "", "/signup");
    }
    router.push("/dashboard");
    router.refresh();
  }

  return (
    <main className="mx-auto w-full max-w-md">
      <p className="text-sm font-medium text-muted-foreground">Photobooth</p>
      <h1 className="mt-3 text-4xl font-semibold tracking-tight">
        {mode.kind === "invite_valid" ? "You’ve been invited to Photobooth" : "Create your account"}
      </h1>
      <p className="mt-3 text-muted-foreground">
        {mode.kind === "invite_valid"
          ? "Set up your studio account to start delivering photos to your clients."
          : "Start delivering photos to your clients."}
      </p>

      {mode.kind === "loading" ? (
        <div className="mt-8 rounded-xl border border-border bg-card p-5 text-sm text-muted-foreground" role="status">
          Checking signup access...
        </div>
      ) : null}

      {mode.kind === "invite_invalid" ? (
        <div className="mt-8 rounded-xl border border-border bg-card p-5" role="status">
          <p className="font-medium text-foreground">This invitation isn&apos;t valid or has expired.</p>
          <p className="mt-2 text-sm text-muted-foreground">
            Ask the person who invited you for a new one.
          </p>
        </div>
      ) : null}

      {mode.kind === "invite_only" ? (
        <div className="mt-8 rounded-xl border border-border bg-card p-5" role="status">
          <p className="font-medium text-foreground">Photobooth is invite-only for now</p>
          <p className="mt-2 text-sm text-muted-foreground">
            You can request early access, or log in if you already have an account.
          </p>
          <div className="mt-4 flex flex-wrap gap-3">
            <Link href="/#early-access" className="inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm font-medium hover:bg-accent">
              Request early access
            </Link>
            <Link href="/login" className="inline-flex min-h-11 items-center rounded-md px-4 text-sm font-medium underline underline-offset-4">
              Log in
            </Link>
          </div>
        </div>
      ) : null}

      {mode.kind === "open" || mode.kind === "invite_valid" ? (
        <form className="mt-8 space-y-5" onSubmit={onSubmit} noValidate>
          <Field id="business_name" label="Business name" error={errors.business_name}>
            <Input id="business_name" autoComplete="organization" required value={form.business_name} onChange={(e) => onBusinessNameChange(e.target.value)} aria-invalid={!!errors.business_name} aria-describedby={errors.business_name ? "business_name-error" : undefined} className="min-h-11" />
          </Field>
          <Field id="email" label="Email" error={errors.email} hint={mode.kind === "invite_valid" ? "This email is set by your invitation." : undefined}>
            <Input id="email" type="email" autoComplete="email" required readOnly={mode.kind === "invite_valid"} value={form.email} onChange={(e) => set("email", e.target.value)} aria-invalid={!!errors.email} aria-describedby={errors.email ? (mode.kind === "invite_valid" ? "email-error email-hint" : "email-error") : (mode.kind === "invite_valid" ? "email-hint" : undefined)} className="min-h-11" />
          </Field>
          <Field id="password" label="Password" error={errors.password} hint="At least 8 characters">
            <div className="relative">
              <Input id="password" type={showPassword ? "text" : "password"} autoComplete="new-password" required value={form.password} onChange={(e) => set("password", e.target.value)} aria-invalid={!!errors.password} aria-describedby={errors.password ? "password-error password-hint" : "password-hint"} className="min-h-11 pr-20" />
              <button type="button" onClick={() => setShowPassword((v) => !v)} className="absolute right-2 top-1/2 -translate-y-1/2 rounded px-2 py-1 text-sm font-medium text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
                {showPassword ? "Hide" : "Show"}
              </button>
            </div>
          </Field>
          <Field id="slug" label="Studio slug" error={errors.slug} hint={`Your link will look like ${form.slug || "your-studio"}.${site.studioDomain}`}>
            <Input id="slug" autoComplete="off" required value={form.slug} onChange={(e) => onSlugChange(e.target.value)} aria-invalid={!!errors.slug} aria-describedby={errors.slug ? "slug-error slug-hint" : "slug-hint"} className="min-h-11" />
          </Field>
          {formError ? <p className="text-sm text-destructive" role="alert">{formError}</p> : null}
          <Button type="submit" disabled={loading} className="min-h-11 w-full">
            {loading ? "Creating account..." : "Create account"}
          </Button>
        </form>
      ) : null}

      <p className="mt-6 text-sm text-muted-foreground">
        Already have an account?{" "}
        <Link href="/login" className="font-medium text-foreground underline underline-offset-4">Log in</Link>
      </p>
    </main>
  );
}

function Field({
  id, label, error, hint, children,
}: {
  id: string; label: string; error?: string; hint?: string; children: React.ReactNode;
}) {
  return (
    <div className="space-y-2">
      <Label htmlFor={id}>{label}</Label>
      {children}
      {hint ? <p id={`${id}-hint`} className="text-xs text-muted-foreground">{hint}</p> : null}
      {error ? <p id={`${id}-error`} className="text-sm text-destructive" role="status">{error}</p> : null}
    </div>
  );
}
