"use client";

import React, { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { useAuth } from "@/providers/auth-provider";
import { useLogin } from "@/hooks/use-auth";
import { useToast } from "@/providers/toast-provider";
import { getDashboardRoute } from "@/lib/auth-routes";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { SkeletonCard } from "@/components/ui/skeleton";

export default function LoginPage() {
  const router = useRouter();
  const { user, isAuthenticated, isLoading } = useAuth();
  const loginMutation = useLogin();
  const toast = useToast();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");

  // Redirect to role dashboard if already logged in
  useEffect(() => {
    if (!isLoading && isAuthenticated && user) {
      router.replace(getDashboardRoute(user.role));
    }
  }, [isLoading, isAuthenticated, user, router]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    try {
      const response = await loginMutation.mutateAsync({ email, password });
      toast.success(
        `Welcome back, ${response.user.name}`,
        "AUTHENTICATED"
      );
      router.replace(getDashboardRoute(response.user.role));
    } catch (err: any) {
      const errorMsg =
        err?.error || err?.message || "Invalid email or password. Please try again.";
      toast.error(errorMsg, "AUTHENTICATION FAILED");
    }
  };

  if (isLoading || (isAuthenticated && user)) {
    return (
      <div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center p-4 font-mono">
        <div className="w-full max-w-md">
          <SkeletonCard />
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center p-4 font-mono">
      <div className="w-full max-w-md">
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>Sign In</CardTitle>
              <span className="text-[10px] bg-black text-white px-2 py-0.5 uppercase tracking-wider font-bold">
                PORTAL AUTH
              </span>
            </div>
            <CardDescription>
              Enter credentials to access your Cadence workspace.
            </CardDescription>
          </CardHeader>

          <form onSubmit={handleSubmit}>
            <CardContent className="space-y-4">
              <Input
                label="Email"
                type="email"
                required
                autoComplete="email"
                placeholder="name@event.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />

              <div className="space-y-1">
                <div className="flex items-center justify-between">
                  <label className="block text-xs uppercase tracking-wider text-neutral-700">
                    Password
                  </label>
                  <Link
                    href="/forgot-password"
                    className="text-[11px] text-neutral-600 hover:text-black underline transition-colors"
                  >
                    Forgot Password?
                  </Link>
                </div>
                <Input
                  type="password"
                  required
                  autoComplete="current-password"
                  placeholder="••••••••"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
              </div>
            </CardContent>

            <CardFooter className="flex flex-col gap-3">
              <Button
                type="submit"
                variant="primary"
                className="w-full"
                isLoading={loginMutation.isPending}
              >
                Sign In
              </Button>

              <div className="flex w-full items-center justify-between text-xs text-neutral-600 pt-2 border-t border-neutral-200">
                <span>First time user?</span>
                <Link
                  href="/setup-password"
                  className="font-bold underline text-black hover:text-neutral-800"
                >
                  Setup Account →
                </Link>
              </div>
            </CardFooter>
          </form>
        </Card>
      </div>
    </div>
  );
}
