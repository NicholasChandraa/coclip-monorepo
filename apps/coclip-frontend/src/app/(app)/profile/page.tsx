"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/contexts/auth-context";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { Loader2, UserCircle, KeyRound, ArrowLeft } from "lucide-react";
import { toast } from "sonner";

export default function ProfilePage() {
  const router = useRouter();
  const { user, authFetch, updateUser } = useAuth();

  // Edit profile state
  const [fullName, setFullName] = useState(user?.full_name ?? "");
  const [email, setEmail] = useState(user?.email ?? "");
  const [username, setUsername] = useState(user?.username ?? "");
  const [isSavingProfile, setIsSavingProfile] = useState(false);

  // Change password state
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [isSavingPassword, setIsSavingPassword] = useState(false);

  const handleSaveProfile = async () => {
    setIsSavingProfile(true);
    try {
      const res = await authFetch("/auth/profile", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          full_name: fullName.trim(),
          email: email.trim(),
          username: username.trim(),
        }),
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        throw new Error(err.detail || "Failed to update profile");
      }
      await updateUser();
      toast.success("Profile updated");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to update profile");
    } finally {
      setIsSavingProfile(false);
    }
  };

  const handleChangePassword = async () => {
    if (newPassword !== confirmPassword) {
      toast.error("New passwords do not match");
      return;
    }
    if (newPassword.length < 8) {
      toast.error("Password must be at least 8 characters");
      return;
    }
    setIsSavingPassword(true);
    try {
      const res = await authFetch("/auth/change-password", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          current_password: currentPassword,
          new_password: newPassword,
        }),
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        throw new Error(err.detail || "Failed to change password");
      }
      setCurrentPassword("");
      setNewPassword("");
      setConfirmPassword("");
      toast.success("Password changed successfully");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to change password");
    } finally {
      setIsSavingPassword(false);
    }
  };

  if (!user) return null;

  const profileChanged =
    fullName.trim() !== user.full_name ||
    email.trim() !== user.email ||
    username.trim() !== user.username;

  return (
    <div className="max-w-6xl mx-auto px-4 sm:px-6 py-8 space-y-6">
      
      
      {/* Header */}
      <div className="flex items-start gap-3">
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

      <div className="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-3">
        <div className="min-w-0 flex-1">
          <h1 className="text-xl font-semibold wrap-break-word leading-snug">Profile</h1>
          <p className="text-muted-foreground mt-1 text-sm">
            Manage your account settings
          </p>
        </div>
      </div>

      {/* Account info card */}
      <Card className="border-border/50">
        <CardHeader className="pb-3">
          <div className="flex items-center gap-3">
            <div className="p-2.5 rounded-full bg-primary/10 border border-primary/20">
              <UserCircle className="w-6 h-6 text-primary" />
            </div>
            <div>
              <CardTitle className="text-base">{user.full_name || user.username}</CardTitle>
              <CardDescription className="text-xs">{user.email}</CardDescription>
            </div>
          </div>
        </CardHeader>
      </Card>

      <Separator />

      {/* Edit profile */}
      <Card className="border-border/50">
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Edit Profile</CardTitle>
          <CardDescription>Update your display name, username, and email</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="fullName" className="text-sm">Full Name</Label>
            <Input
              id="fullName"
              value={fullName}
              onChange={(e) => setFullName(e.target.value)}
              placeholder="Your full name"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="username" className="text-sm">Username</Label>
            <Input
              id="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="username"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="email" className="text-sm">Email</Label>
            <Input
              id="email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="you@example.com"
            />
          </div>
          <Button
            onClick={handleSaveProfile}
            disabled={isSavingProfile || !profileChanged}
            className="w-full sm:w-auto"
          >
            {isSavingProfile ? (
              <>
                <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                Saving…
              </>
            ) : (
              "Save Changes"
            )}
          </Button>
        </CardContent>
      </Card>

      <Separator />

      {/* Change password */}
      <Card className="border-border/50">
        <CardHeader className="pb-3">
          <div className="flex items-center gap-2">
            <KeyRound className="w-4 h-4 text-muted-foreground" />
            <CardTitle className="text-base">Change Password</CardTitle>
          </div>
          <CardDescription>Minimum 8 characters</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="currentPassword" className="text-sm">Current Password</Label>
            <Input
              id="currentPassword"
              type="password"
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
              placeholder="••••••••"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="newPassword" className="text-sm">New Password</Label>
            <Input
              id="newPassword"
              type="password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              placeholder="••••••••"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="confirmPassword" className="text-sm">Confirm New Password</Label>
            <Input
              id="confirmPassword"
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              placeholder="••••••••"
              onKeyDown={(e) => e.key === "Enter" && handleChangePassword()}
            />
          </div>
          <Button
            onClick={handleChangePassword}
            disabled={isSavingPassword || !currentPassword || !newPassword || !confirmPassword}
            variant="destructive"
            className="w-full sm:w-auto"
          >
            {isSavingPassword ? (
              <>
                <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                Updating…
              </>
            ) : (
              "Change Password"
            )}
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
