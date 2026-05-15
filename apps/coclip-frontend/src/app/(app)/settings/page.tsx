"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useAuth } from "@/contexts/auth-context";
import {
  getSocialAccounts,
  getYouTubeOAuthUrl,
  disconnectAccount,
  type SocialAccount,
} from "@/lib/social-api";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { ArrowLeft, Info, Loader2, Youtube } from "lucide-react";
import { toast } from "sonner";

const TikTokIcon = ({ className }: { className?: string }) => (
  <svg
    role="img"
    viewBox="0 0 24 24"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
    fill="currentColor"
  >
    <path d="M12.525.02c1.31-.02 2.61-.01 3.91-.02.08 1.53.63 3.09 1.75 4.17 1.12 1.11 2.7 1.62 4.24 1.79v4.03c-1.44-.05-2.89-.35-4.2-.97-.57-.26-1.1-.59-1.62-.93-.01 2.92.01 5.84-.02 8.75-.08 2.23-1.13 4.41-2.9 5.67-1.8 1.3-4.25 1.69-6.38 1.05-2.18-.65-4.04-2.31-4.79-4.43-.84-2.37-.32-5.2 1.4-7.07 1.63-1.78 4.28-2.5 6.6-1.92v4.06c-1.07-.35-2.32-.23-3.21.43-.87.64-1.32 1.74-1.2 2.82.12 1.14.99 2.12 2.1 2.45 1.17.34 2.53.1 3.46-.66.86-.7 1.33-1.81 1.33-2.92V.02z" />
  </svg>
);

export default function SettingsPage() {

  const router = useRouter();
  const { authFetch } = useAuth();
  const searchParams = useSearchParams();

  const [accounts, setAccounts] = useState<SocialAccount[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isConnecting, setIsConnecting] = useState(false);
  const [disconnecting, setDisconnecting] = useState<string | null>(null);

  const fetchAccounts = useCallback(async () => {
    try {
      const data = await getSocialAccounts(authFetch);
      setAccounts(data);
    } catch {
      toast.error("Failed to load connected accounts");
    } finally {
      setIsLoading(false);
    }
  }, [authFetch]);

  useEffect(() => {
    fetchAccounts();
  }, [fetchAccounts]);

  // Handle redirect back from OAuth
  useEffect(() => {
    const connected = searchParams.get("connected");
    const error = searchParams.get("error");
    if (connected) {
      toast.success(`${connected} connected successfully!`);
      fetchAccounts();
    }
    if (error) {
      toast.error(`Failed to connect: ${error.replace(/_/g, " ")}`);
    }
  }, [searchParams, fetchAccounts]);

  const handleConnect = async (platform: string) => {
    setIsConnecting(true);
    try {
      if (platform === "youtube") {
        const url = await getYouTubeOAuthUrl(authFetch);
        window.location.href = url;
      }
    } catch {
      toast.error(`Failed to start ${platform} connection`);
      setIsConnecting(false);
    }
  };

  const handleDisconnect = async (platform: string) => {
    setDisconnecting(platform);
    try {
      await disconnectAccount(authFetch, platform);
      setAccounts((prev) => prev.filter((a) => a.platform !== platform));
      toast.success(`${platform} disconnected`);
    } catch {
      toast.error(`Failed to disconnect ${platform}`);
    } finally {
      setDisconnecting(null);
    }
  };

  const youtubeAccount = accounts.find((a) => a.platform === "youtube");

  return (
    <div className="max-w-2xl mx-auto px-4 py-8 space-y-8">
      <div>
        {/* Header */}
        <div className="flex items-start">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => router.push("/dashboard")}
            className="text-muted-foreground hover:text-foreground mt-0.5 -ml-2 cursor-pointer"
          >
            <ArrowLeft className="w-4 h-4 mr-1" />
            Back
          </Button>
        </div>
        <div>
          <h1 className="text-2xl font-bold">Settings</h1>
          <p className="text-muted-foreground text-sm mt-1">Manage your connected accounts</p>
        </div>
      </div>

      <div className="space-y-3">
        <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
          Connected Accounts
        </h2>

        {isLoading ? (
          <div className="flex items-center gap-2 text-muted-foreground text-sm py-4">
            <Loader2 className="w-4 h-4 animate-spin" />
            Loading…
          </div>
        ) : (
          <div className="border border-border rounded-lg divide-y divide-border">
            <div className="flex items-center justify-between px-4 py-3.5">
              <div className="flex items-center gap-3">
                <Youtube className="w-5 h-5 text-red-500" />
                <div>
                  <p className="text-sm font-medium">YouTube</p>
                  {youtubeAccount ? (
                    <p className="text-xs text-muted-foreground">
                      {youtubeAccount.platform_username}
                    </p>
                  ) : (
                    <p className="text-xs text-muted-foreground">Not connected</p>
                  )}
                </div>
              </div>

              {youtubeAccount ? (
                <div className="flex items-center gap-2">
                  <Badge
                    variant="outline"
                    className="text-xs bg-emerald-500/10 text-emerald-400 border-emerald-500/20"
                  >
                    Connected
                  </Badge>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="text-xs h-7 text-muted-foreground hover:text-destructive"
                    disabled={disconnecting === "youtube"}
                    onClick={() => handleDisconnect("youtube")}
                  >
                    {disconnecting === "youtube" ? (
                      <Loader2 className="w-3 h-3 animate-spin" />
                    ) : (
                      "Disconnect"
                    )}
                  </Button>
                </div>
              ) : (
                <Button
                  size="sm"
                  className="h-7 text-xs"
                  disabled={isConnecting}
                  onClick={() => handleConnect("youtube")}
                >
                  {isConnecting ? (
                    <>
                      <Loader2 className="w-3 h-3 mr-1.5 animate-spin" />
                      Connecting…
                    </>
                  ) : (
                    "Connect"
                  )}
                </Button>
              )}
            </div>

            {/* TikTok */}
            <div className="flex items-center gap-3 px-4 py-3.5 border-t border-border">
              <div className="bg-black text-white p-1 rounded-sm w-6 h-6 flex items-center justify-center shrink-0">
                <TikTokIcon className="w-4 h-4" />
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium">TikTok</p>
                <p className="text-xs text-muted-foreground flex items-center gap-1">
                  <Info className="w-3 h-3 shrink-0" />
                  Connect via Chrome when uploading clips
                </p>
              </div>
            </div>

          </div>
        )}
      </div>
    </div>
  );
}
