"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { QRCodeSVG } from "qrcode.react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { messageForAdminError } from "@/lib/admin/errors";
import {
  adminLogin,
  adminTOTPSetupConfirm,
  adminTOTPSetupStart,
  adminTOTPVerify,
} from "@/lib/api/admin";

type Step =
  | { kind: "password" }
  | { kind: "totp" }
  | { kind: "setup"; otpauthUri: string; secret: string }
  | { kind: "recovery-codes"; codes: string[] };

export function AdminLoginForm() {
  const router = useRouter();
  const [step, setStep] = useState<Step>({ kind: "password" });
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [code, setCode] = useState("");
  const [recoveryMode, setRecoveryMode] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);
  const [savedCodes, setSavedCodes] = useState(false);

  async function onPassword(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setPending(true);
    try {
      const res = await adminLogin({ email, password });
      if (!res.ok) {
        setError(messageForAdminError(res));
        return;
      }
      if (res.data.next === "ok") {
        router.push("/admin");
        router.refresh();
        return;
      }
      if (res.data.next === "totp") {
        setStep({ kind: "totp" });
        setCode("");
        setRecoveryMode(false);
        return;
      }
      const setup = await adminTOTPSetupStart();
      if (!setup.ok) {
        setError(messageForAdminError(setup));
        return;
      }
      setStep({ kind: "setup", otpauthUri: setup.data.otpauthUri, secret: setup.data.secret });
      setCode("");
    } finally {
      setPending(false);
    }
  }

  async function onVerify(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setPending(true);
    try {
      const res = await adminTOTPVerify(recoveryMode ? { recoveryCode: code } : { code });
      if (!res.ok) {
        setError(messageForAdminError(res));
        return;
      }
      router.push("/admin");
      router.refresh();
    } finally {
      setPending(false);
    }
  }

  async function onSetupConfirm(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setPending(true);
    try {
      const res = await adminTOTPSetupConfirm({ code });
      if (!res.ok) {
        setError(messageForAdminError(res));
        return;
      }
      setStep({ kind: "recovery-codes", codes: res.data.recoveryCodes });
      setSavedCodes(false);
    } finally {
      setPending(false);
    }
  }

  if (step.kind === "recovery-codes") {
    return (
      <div className="space-y-4">
        <div>
          <h2 className="text-lg font-semibold">Recovery codes</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            Save these recovery codes now. They are shown once and cannot be shown again.
          </p>
        </div>
        <div className="rounded-lg border border-border bg-muted/30 p-4">
          <ul className="grid grid-cols-1 gap-2 font-mono text-sm sm:grid-cols-2">
            {step.codes.map((c) => <li key={c}>{c}</li>)}
          </ul>
        </div>
        <div className="flex flex-wrap gap-3">
          <Button type="button" variant="outline" className="min-h-11" onClick={() => navigator.clipboard.writeText(step.codes.join("\n"))}>
            Copy codes
          </Button>
        </div>
        <label className="flex min-h-11 items-center gap-3 text-sm">
          <input type="checkbox" checked={savedCodes} onChange={(e) => setSavedCodes(e.target.checked)} />
          I saved these recovery codes
        </label>
        <Button
          className="min-h-11 w-full"
          disabled={!savedCodes}
          onClick={() => {
            router.push("/admin");
            router.refresh();
          }}
        >
          Continue to admin
        </Button>
      </div>
    );
  }

  if (step.kind === "setup") {
    return (
      <form onSubmit={onSetupConfirm} className="space-y-4">
        <div>
          <h2 className="text-lg font-semibold">Set up authenticator</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            Scan the QR code or enter the secret in your authenticator app, then confirm with a code.
          </p>
        </div>
        <div className="flex justify-center rounded-lg border border-border bg-background p-4">
          <QRCodeSVG value={step.otpauthUri} size={180} includeMargin />
        </div>
        <div className="space-y-2">
          <Label>Setup secret</Label>
          <div className="rounded-md border border-border bg-muted/30 p-3 font-mono text-xs break-all">{step.secret}</div>
        </div>
        <div className="space-y-2">
          <Label htmlFor="setup-code">Authenticator code</Label>
          <Input id="setup-code" value={code} onChange={(e) => setCode(e.target.value)} autoComplete="one-time-code" inputMode="numeric" className="min-h-11" required />
        </div>
        {error ? <p className="text-sm text-destructive" role="status">{error}</p> : null}
        <Button type="submit" disabled={pending} className="min-h-11 w-full">{pending ? "Confirming..." : "Confirm setup"}</Button>
      </form>
    );
  }

  if (step.kind === "totp") {
    return (
      <form onSubmit={onVerify} className="space-y-4">
        <div>
          <h2 className="text-lg font-semibold">{recoveryMode ? "Recovery code" : "Authenticator code"}</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            {recoveryMode ? "Enter one of your saved recovery codes." : "Enter the code from your authenticator app."}
          </p>
        </div>
        <div className="space-y-2">
          <Label htmlFor="totp-code">{recoveryMode ? "Recovery code" : "Code"}</Label>
          <Input id="totp-code" value={code} onChange={(e) => setCode(e.target.value)} autoComplete={recoveryMode ? "off" : "one-time-code"} className="min-h-11" required />
        </div>
        {error ? <p className="text-sm text-destructive" role="status">{error}</p> : null}
        <Button type="submit" disabled={pending} className="min-h-11 w-full">{pending ? "Verifying..." : "Verify"}</Button>
        <Button type="button" variant="ghost" className="min-h-11 w-full" onClick={() => { setRecoveryMode((v) => !v); setCode(""); setError(null); }}>
          {recoveryMode ? "Use authenticator code" : "Use a recovery code"}
        </Button>
      </form>
    );
  }

  return (
    <form onSubmit={onPassword} className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="admin-email">Email</Label>
        <Input id="admin-email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="email" className="min-h-11" required />
      </div>
      <div className="space-y-2">
        <Label htmlFor="admin-password">Password</Label>
        <Input id="admin-password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="current-password" className="min-h-11" required />
      </div>
      {error ? <p className="text-sm text-destructive" role="status">{error}</p> : null}
      <Button type="submit" disabled={pending} className="min-h-11 w-full">{pending ? "Signing in..." : "Sign in"}</Button>
    </form>
  );
}
