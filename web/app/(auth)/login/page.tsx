"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { login, me } from "@/lib/auth/client";
import { mapAuthError, validateEmail } from "@/lib/validation/auth";

export default function LoginPage() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    me().then((result) => {
      if (result.ok) {
        router.replace("/dashboard");
        router.refresh();
      }
    });
  }, [router]);

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError(null);
    setFieldError(null);
    const emailValidation = validateEmail(email);
    if (!emailValidation.ok) {
      setFieldError(emailValidation.message);
      document.getElementById("email")?.focus();
      return;
    }
    if (!password) {
      setFieldError("Enter your password.");
      document.getElementById("password")?.focus();
      return;
    }

    setLoading(true);
    const result = await login({ email: email.trim(), password });
    setLoading(false);
    if (!result.ok) {
      setError(mapAuthError(result));
      return;
    }
    router.push("/dashboard");
    router.refresh();
  }

  return (
    <main className="mx-auto w-full max-w-md">
      <p className="text-sm font-medium text-muted-foreground">Photobooth</p>
      <h1 className="mt-3 text-4xl font-semibold tracking-tight">Log in</h1>
      <p className="mt-3 text-muted-foreground">Access your dashboard and manage your shoots.</p>

      <form className="mt-8 space-y-5" onSubmit={onSubmit} noValidate>
        <div className="space-y-2">
          <Label htmlFor="email">Email</Label>
          <Input
            id="email"
            type="email"
            autoComplete="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            aria-invalid={!!fieldError}
            aria-describedby={fieldError ? "email-error" : undefined}
            className="min-h-11"
          />
          {fieldError ? (
            <p id="email-error" className="text-sm text-destructive" role="status">
              {fieldError}
            </p>
          ) : null}
        </div>
        <div className="space-y-2">
          <Label htmlFor="password">Password</Label>
          <Input
            id="password"
            type="password"
            autoComplete="current-password"
            required
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="min-h-11"
          />
        </div>
        {error ? <p className="text-sm text-destructive" role="alert">{error}</p> : null}
        <Button type="submit" disabled={loading} className="min-h-11 w-full">
          {loading ? "Logging in..." : "Log in"}
        </Button>
      </form>

      <p className="mt-6 text-sm text-muted-foreground">
        Don&apos;t have an account?{" "}
        <Link href="/signup" className="font-medium text-foreground underline underline-offset-4">
          Sign up
        </Link>
      </p>
    </main>
  );
}
