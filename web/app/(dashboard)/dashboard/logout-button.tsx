"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { authApi } from "@/lib/api/auth";

export function LogoutButton() {
  const router = useRouter();
  const [loading, setLoading] = useState(false);

  async function onLogout() {
    setLoading(true);
    await authApi.logout();
    router.push("/login");
    router.refresh();
  }

  return (
    <Button variant="outline" onClick={onLogout} disabled={loading} className="min-h-11">
      {loading ? "Logging out..." : "Log out"}
    </Button>
  );
}
