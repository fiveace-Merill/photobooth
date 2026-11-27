"use client";

import { useState, type FormEvent } from "react";
import Link from "next/link";
import { landing, photographyTypes, shootsPerMonthOptions } from "@/lib/content/landing";
import { site } from "@/lib/site";
import { waitlistApi } from "@/lib/api/waitlist";
import { validateWaitlistInput, type WaitlistInput } from "@/lib/validation/waitlist";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

type FieldErrors = Partial<Record<keyof WaitlistInput, string>>;

const initial: WaitlistInput = {
  name: "",
  contact: "",
  photographyType: "",
  shootsPerMonth: "",
  consent: false,
  website: "",
};

export function EarlyAccessForm() {
  const [input, setInput] = useState<WaitlistInput>(initial);
  const [errors, setErrors] = useState<FieldErrors>({});
  const [pending, setPending] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  function setField<K extends keyof WaitlistInput>(key: K, value: WaitlistInput[K]) {
    setInput((prev) => ({ ...prev, [key]: value }));
    setErrors((prev) => ({ ...prev, [key]: undefined }));
  }

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setSubmitError(null);
    const result = validateWaitlistInput(input);
    if (!result.ok) {
      if (result.silent) {
        setSubmitted(true);
        return;
      }
      setErrors(result.errors);
      const first = (["name", "contact", "photographyType", "shootsPerMonth", "consent"] as const).find(
        (k) => result.errors[k],
      );
      if (first) document.getElementById(first)?.focus();
      return;
    }

    setPending(true);
    try {
      const res = await waitlistApi.join(result.data);
      if (res.ok) {
        setSubmitted(true);
        return;
      }
      setSubmitError(res.message);
    } finally {
      setPending(false);
    }
  }

  if (submitted) {
    return (
      <div className="rounded-2xl border border-border bg-card p-6 sm:p-8" role="status" aria-live="polite">
        <p className="text-xl font-semibold text-foreground">{landing.earlyAccess.success}</p>
        <p className="mt-3 text-sm text-muted-foreground">
          We’ll reach out using the contact details you shared.
        </p>
      </div>
    );
  }

  return (
    <form onSubmit={onSubmit} noValidate className="space-y-5">
      <div className="absolute -left-[10000px] top-auto h-px w-px overflow-hidden" aria-hidden="true">
        <label htmlFor="website">Website</label>
        <input
          id="website"
          name="website"
          tabIndex={-1}
          autoComplete="off"
          value={input.website}
          onChange={(e) => setField("website", e.target.value)}
        />
      </div>

      <Field label="Name" htmlFor="name" error={errors.name} required>
        <Input
          id="name"
          name="name"
          autoComplete="name"
          value={input.name}
          onChange={(e) => setField("name", e.target.value)}
          aria-invalid={!!errors.name}
          aria-describedby={errors.name ? "name-error" : undefined}
          className="min-h-11"
        />
      </Field>

      <Field label="Email or phone" htmlFor="contact" error={errors.contact} required>
        <Input
          id="contact"
          name="contact"
          autoComplete="email tel"
          inputMode="email"
          value={input.contact}
          onChange={(e) => setField("contact", e.target.value)}
          aria-invalid={!!errors.contact}
          aria-describedby={errors.contact ? "contact-error" : undefined}
          className="min-h-11"
        />
      </Field>

      <Field label="Main type of photography" htmlFor="photographyType" error={errors.photographyType} required>
        <Select value={input.photographyType} onValueChange={(v) => setField("photographyType", v ?? "")}>
          <SelectTrigger
            id="photographyType"
            aria-invalid={!!errors.photographyType}
            aria-describedby={errors.photographyType ? "photographyType-error" : undefined}
            className="min-h-11 w-full"
          >
            <SelectValue placeholder="Select one" />
          </SelectTrigger>
          <SelectContent>
            {photographyTypes.map((option) => (
              <SelectItem key={option} value={option}>
                {option}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Field>

      <Field label="Shoots per month" htmlFor="shootsPerMonth" error={errors.shootsPerMonth} required>
        <Select value={input.shootsPerMonth} onValueChange={(v) => setField("shootsPerMonth", v ?? "")}>
          <SelectTrigger
            id="shootsPerMonth"
            aria-invalid={!!errors.shootsPerMonth}
            aria-describedby={errors.shootsPerMonth ? "shootsPerMonth-error" : undefined}
            className="min-h-11 w-full"
          >
            <SelectValue placeholder="Select one" />
          </SelectTrigger>
          <SelectContent>
            {shootsPerMonthOptions.map((option) => (
              <SelectItem key={option} value={option}>
                {option}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Field>

      <div className="space-y-2">
        <div className="flex items-start gap-3">
          <Checkbox
            id="consent"
            checked={input.consent}
            onCheckedChange={(checked) => setField("consent", checked === true)}
            aria-invalid={!!errors.consent}
            aria-describedby={errors.consent ? "consent-error" : undefined}
            className="mt-0.5"
          />
          <Label htmlFor="consent" className="text-sm leading-relaxed text-foreground">
            I agree to be contacted about Photobooth{" "}
            <Link href="/privacy" className="underline underline-offset-4 hover:text-foreground/80">
              Privacy
            </Link>
          </Label>
        </div>
        {errors.consent ? (
          <p id="consent-error" className="text-sm text-destructive" role="status">
            {errors.consent}
          </p>
        ) : null}
      </div>

      {submitError ? (
        <p className="text-sm text-destructive" role="status" aria-live="polite">
          {submitError}
        </p>
      ) : null}

      <Button type="submit" disabled={pending} className="min-h-11 w-full sm:w-auto">
        {pending ? "Sending..." : "Get early access"}
      </Button>

      <p className="text-sm text-muted-foreground">
        Or contact us at{" "}
        <a href={`mailto:${site.contactEmail}`} className="underline underline-offset-4 hover:text-foreground">
          {site.contactEmail}
        </a>{" "}
        or{" "}
        <a href={`https://wa.me/${site.whatsappNumber.replace(/\+/g, "")}`} className="underline underline-offset-4 hover:text-foreground">
          WhatsApp {site.whatsappDisplay}
        </a>
        .
      </p>
    </form>
  );
}

function Field({
  label,
  htmlFor,
  error,
  required,
  children,
}: {
  label: string;
  htmlFor: string;
  error?: string;
  required?: boolean;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-2">
      <Label htmlFor={htmlFor} className="text-sm font-medium">
        {label} {required ? <span aria-hidden="true">*</span> : null}
      </Label>
      {children}
      {error ? (
        <p id={`${htmlFor}-error`} className="text-sm text-destructive" role="status">
          {error}
        </p>
      ) : null}
    </div>
  );
}
