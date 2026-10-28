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
import { Badge } from "@/components/ui/badge";

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
      <div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center p-4 font-sans">
        <div className="w-full max-w-md">
          <SkeletonCard />
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center p-4 font-sans">
      <div className="w-full max-w-md">
        <div className="translucent-container p-2 sm:p-3">
          <Card className="border border-[var(--card-border)] shadow-none">
            <CardHeader className="pb-4">
              <div className="flex items-center justify-between">
                <CardTitle className="text-xl font-bold tracking-tight text-[var(--ink-primary)]">
                  Sign In
                </CardTitle>
                <Badge variant="blue" className="text-[10px] tracking-wider uppercase font-semibold">
                  PORTAL AUTH
                </Badge>
              </div>
              <CardDescription className="text-xs text-[var(--ink-secondary)] mt-1">
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
                    <label className="block text-xs font-semibold text-[var(--ink-secondary)]">
                      Password
                    </label>
                    <Link
                      href="/forgot-password"
                      className="text-xs font-medium text-[var(--color-blue)] hover:underline transition-colors"
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

              <CardFooter className="flex flex-col gap-3 pt-3">
                <Button
                  type="submit"
                  variant="primary"
                  className="w-full h-11 text-sm font-semibold"
                  isLoading={loginMutation.isPending}
                >
                  Sign In
                </Button>

                <div className="flex w-full items-center justify-between text-xs text-[var(--ink-secondary)] pt-3 border-t border-[var(--card-line)]">
                  <span>First time user?</span>
                  <Link
                    href="/setup-password"
                    className="font-semibold text-[var(--ink-primary)] hover:text-[var(--color-blue)] transition-colors"
                  >
                    Setup Account →
                  </Link>
                </div>
              </CardFooter>
            </form>
          </Card>
        </div>
      </div>
    </div>
  );
}
