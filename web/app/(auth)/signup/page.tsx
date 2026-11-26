"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { me, signup } from "@/lib/auth/client";
import { site } from "@/lib/site";
import {
  mapAuthError,
  suggestSlug,
  validateEmail,
  validatePassword,
  validateSlug,
} from "@/lib/validation/auth";

type FieldErrors = Partial<Record<"business_name" | "email" | "password" | "slug", string>>;

export default function SignupPage() {
  const router = useRouter();
  const [form, setForm] = useState({ business_name: "", email: "", password: "", slug: "" });
  const [slugEdited, setSlugEdited] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [errors, setErrors] = useState<FieldErrors>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [signupClosed, setSignupClosed] = useState(false);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    me().then((result) => {
      if (result.ok) {
        router.replace("/dashboard");
        router.refresh();
      }
    });
  }, [router]);

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
    setSignupClosed(false);
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
    });
    setLoading(false);
    if (!result.ok) {
      const mapped = mapAuthError(result);
      if (mapped === "signup_closed") {
        setSignupClosed(true);
        return;
      }
      if (result.code === "email_taken") setErrors({ email: mapped });
      else if (result.code === "slug_taken") setErrors({ slug: mapped });
      else setFormError(mapped);
      return;
    }
    router.push("/dashboard");
    router.refresh();
  }

  return (
    <main className="mx-auto w-full max-w-md">
      <p className="text-sm font-medium text-muted-foreground">Photobooth</p>
      <h1 className="mt-3 text-4xl font-semibold tracking-tight">Create your account</h1>
      <p className="mt-3 text-muted-foreground">Start delivering photos to your clients.</p>

      {signupClosed ? (
        <div className="mt-8 rounded-xl border border-border bg-card p-5" role="status">
          <p className="font-medium text-foreground">Signups are invite-only right now.</p>
          <p className="mt-2 text-sm text-muted-foreground">
            You can request early access and we’ll be in touch when a spot opens.
          </p>
          <Link href="/#early-access" className="mt-4 inline-flex min-h-11 items-center text-sm font-medium underline underline-offset-4">
            Back to early access
          </Link>
        </div>
      ) : (
        <form className="mt-8 space-y-5" onSubmit={onSubmit} noValidate>
          <Field id="business_name" label="Business name" error={errors.business_name}>
            <Input id="business_name" autoComplete="organization" required value={form.business_name} onChange={(e) => onBusinessNameChange(e.target.value)} aria-invalid={!!errors.business_name} aria-describedby={errors.business_name ? "business_name-error" : undefined} className="min-h-11" />
          </Field>
          <Field id="email" label="Email" error={errors.email}>
            <Input id="email" type="email" autoComplete="email" required value={form.email} onChange={(e) => set("email", e.target.value)} aria-invalid={!!errors.email} aria-describedby={errors.email ? "email-error" : undefined} className="min-h-11" />
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
      )}

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
