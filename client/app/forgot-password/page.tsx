"use client";

import React, { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { useAuth } from "@/providers/auth-provider";
import { useForgotPassword } from "@/hooks/use-auth";
import { useToast } from "@/providers/toast-provider";
import { getDashboardRoute } from "@/lib/auth-routes";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { SkeletonCard } from "@/components/ui/skeleton";
import { Alert } from "@/components/ui/alert";

export default function ForgotPasswordPage() {
  const router = useRouter();
  const { user, isAuthenticated, isLoading } = useAuth();
  const forgotPasswordMutation = useForgotPassword();
  const toast = useToast();

  const [email, setEmail] = useState("");
  const [isSuccess, setIsSuccess] = useState(false);

  useEffect(() => {
    if (!isLoading && isAuthenticated && user) {
      router.replace(getDashboardRoute(user.role));
    }
  }, [isLoading, isAuthenticated, user, router]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    try {
      const response = await forgotPasswordMutation.mutateAsync({ email });
      setIsSuccess(true);
      toast.success(
        response.message || "Password reset link sent to your email.",
        "DISPATCHED"
      );
    } catch (err: any) {
      const errorMsg =
        err?.error || err?.message || "Failed to process password reset request.";
      toast.error(errorMsg, "REQUEST FAILED");
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
              <CardTitle>Reset Password</CardTitle>
              <span className="text-[10px] bg-black text-white px-2 py-0.5 uppercase tracking-wider font-bold">
                RECOVERY
              </span>
            </div>
            <CardDescription>
              Enter your registered email address to receive recovery instructions.
            </CardDescription>
          </CardHeader>

          {isSuccess ? (
            <div>
              <CardContent className="space-y-4">
                <Alert variant="success" title="RECOVERY EMAIL SENT">
                  If an account exists for <strong>{email}</strong>, a secure password
                  reset link has been sent. Please check your inbox and follow the instructions.
                </Alert>
              </CardContent>

              <CardFooter className="flex flex-col gap-2 pt-2">
                <Button
                  variant="outline"
                  className="w-full"
                  onClick={() => setIsSuccess(false)}
                >
                  Send to Another Email
                </Button>

                <Link href="/login" className="w-full">
                  <Button variant="primary" className="w-full">
                    Return to Login
                  </Button>
                </Link>
              </CardFooter>
            </div>
          ) : (
            <form onSubmit={handleSubmit}>
              <CardContent className="space-y-4">
                <Input
                  label="Registered Email"
                  type="email"
                  required
                  autoComplete="email"
                  placeholder="name@event.com"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  helperText="Password reset token will be delivered via secure SMTP."
                />
              </CardContent>

              <CardFooter className="flex flex-col gap-3">
                <Button
                  type="submit"
                  variant="primary"
                  className="w-full"
                  isLoading={forgotPasswordMutation.isPending}
                >
                  Send Recovery Link
                </Button>

                <div className="flex w-full justify-center text-xs text-neutral-600 pt-1">
                  <Link
                    href="/login"
                    className="hover:text-black underline transition-colors"
                  >
                    ← Back to Sign In
                  </Link>
                </div>
              </CardFooter>
            </form>
          )}
        </Card>
      </div>
    </div>
  );
}
