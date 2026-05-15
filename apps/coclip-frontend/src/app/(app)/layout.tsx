"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Image from "next/image";
import { useAuth } from "@/contexts/auth-context";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { Loader2, LogOut, Settings, UserCircle } from "lucide-react";
import { toast } from "sonner";
import { ACTIVE_JOB_KEY } from "@/contexts/job-context";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const { user, isLoading, logout } = useAuth();
  const [showLogoutConfirm, setShowLogoutConfirm] = useState(false);

  useEffect(() => {
    if (!isLoading && !user) {
      router.replace("/login");
    }
  }, [user, isLoading, router]);

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <Loader2 className="w-6 h-6 animate-spin text-primary" />
      </div>
    );
  }

  if (!user) return null;

  const handleLogoutClick = () => {
    const hasActiveJob = !!localStorage.getItem(ACTIVE_JOB_KEY);
    if (hasActiveJob) {
      setShowLogoutConfirm(true);
    } else {
      performLogout();
    }
  };

  const performLogout = async () => {
    setShowLogoutConfirm(false);
    await logout();
    toast.success("Signed out");
    router.replace("/login");
  };

  return (
    <div className="min-h-screen flex flex-col bg-background">
      {/* Logout confirm dialog */}
      {showLogoutConfirm && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-background border border-border rounded-xl shadow-xl w-full max-w-sm p-6 space-y-4">
            <div className="space-y-1">
              <h2 className="text-base font-semibold">Sign out?</h2>
              <p className="text-sm text-muted-foreground">
                A video is still being processed. Are you sure you want to sign out? You can resume tracking when you log back in.
              </p>
            </div>
            <div className="flex gap-2 justify-end pt-2">
              <Button variant="outline" size="sm" onClick={() => setShowLogoutConfirm(false)}>
                Cancel
              </Button>
              <Button variant="destructive" size="sm" onClick={performLogout}>
                Sign Out
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Navbar */}
      <header className="sticky top-0 z-50 border-b border-border/40 bg-background/80 backdrop-blur-md">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 h-14 flex items-center justify-between">
          {/* Logo */}
          <button
            className="flex items-center gap-2 cursor-pointer"
            onClick={() => router.push("/dashboard")}
          >
            <Image src="/logo_2s.png" alt="CoClip Logo" width={32} height={32} className="rounded-md" />
            <span className="font-bold text-[15px] tracking-tight">CoClip</span>
          </button>

          {/* Right side */}
          <div className="flex items-center gap-1">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => router.push("/profile")}
              className="text-muted-foreground hover:text-foreground h-8 px-2 hidden sm:flex items-center gap-1.5 cursor-pointer"
            >
              <UserCircle className="w-3.5 h-3.5" />
              <span className="text-xs">{user.username}</span>
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => router.push("/settings")}
              className="text-muted-foreground hover:text-foreground h-8 px-2 hidden sm:flex items-center gap-1.5 cursor-pointer"
            >
              <Settings className="w-3.5 h-3.5" />
              <span className="text-xs">Settings</span>
            </Button>
            <Separator orientation="vertical" className="h-4 hidden sm:block" />
            <Button
              variant="ghost"
              size="sm"
              onClick={handleLogoutClick}
              className="text-muted-foreground hover:text-foreground h-8 px-2 cursor-pointer"
            >
              <LogOut className="w-3.5 h-3.5" />
              <span className="ml-1.5 hidden sm:inline text-xs">Sign out</span>
            </Button>
          </div>
        </div>
      </header>

      <main className="flex-1">{children}</main>
    </div>
  );
}
