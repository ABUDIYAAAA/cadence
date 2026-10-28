"use client";

import React, { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { useLogin } from "@/hooks/use-auth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Alert } from "@/components/ui/alert";

export default function LoginPage() {
  const router = useRouter();
  const loginMutation = useLogin();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrorMessage(null);

    try {
      await loginMutation.mutateAsync({ email, password });
      router.push("/");
    } catch (err: any) {
      setErrorMessage(
        err?.error || err?.message || "Authentication failed. Check your credentials."
      );
    }
  };

  return (
    <div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center p-4 font-mono">
      <div className="w-full max-w-md">
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>Sign In</CardTitle>
              <span className="text-[10px] bg-black text-white px-2 py-0.5 uppercase">AUTH</span>
            </div>
            <CardDescription>
              Enter credentials to access the Cadence system.
            </CardDescription>
          </CardHeader>

          <form onSubmit={handleSubmit}>
            <CardContent className="space-y-4">
              {errorMessage && (
                <Alert variant="error" title="LOGIN ERROR">
                  {errorMessage}
                </Alert>
              )}

              <Input
                label="Email"
                type="email"
                required
                autoComplete="email"
                placeholder="user@example.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />

              <Input
                label="Password"
                type="password"
                required
                autoComplete="current-password"
                placeholder="••••••••"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
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

              <div className="flex w-full justify-between text-xs text-neutral-500 pt-1">
                <Link
                  href="/"
                  className="hover:text-black underline transition-colors"
                >
                  ← Back to Home
                </Link>
                <span>Cadence v1.0</span>
              </div>
            </CardFooter>
          </form>
        </Card>
      </div>
    </div>
  );
}
