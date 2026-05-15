"use client";

import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Scissors } from "lucide-react";

export default function NotFound() {
  const router = useRouter();

  return (
    <div className="min-h-screen flex flex-col items-center justify-center bg-background text-center px-4">
      <div className="p-3 rounded-xl bg-primary/10 border border-primary/20 mb-6">
        <Scissors className="w-8 h-8 text-primary" />
      </div>
      <p className="text-8xl font-black tracking-tighter text-primary mb-4">404</p>
      <h1 className="text-xl font-semibold mb-2">Page not found</h1>
      <p className="text-sm text-muted-foreground mb-8 max-w-xs">
        Looks like this clip got cut on the cutting room floor.
      </p>
      <div className="flex gap-3">
        <Button variant="outline" onClick={() => router.back()}>
          Go back
        </Button>
        <Button onClick={() => router.push("/dashboard")}>
          Dashboard
        </Button>
      </div>
    </div>
  );
}
