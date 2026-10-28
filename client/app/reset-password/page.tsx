"use client";

import React, { Suspense, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import { useResetPassword } from "@/hooks/use-auth";
import { useToast } from "@/providers/toast-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { SkeletonCard } from "@/components/ui/skeleton";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";

function ResetPasswordForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const urlToken = searchParams.get("token") || "";

  const resetPasswordMutation = useResetPassword();
  const toast = useToast();

  const [manualToken, setManualToken] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [validationError, setValidationError] = useState<string | null>(null);
  const [isSuccess, setIsSuccess] = useState(false);

  // Active token is either from URL query param or manual entry
  const effectiveToken = urlToken.trim() || manualToken.trim();
  const hasUrlToken = Boolean(urlToken.trim());

  useEffect(() => {
    if (urlToken) {
      setManualToken(urlToken);
    }
  }, [urlToken]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setValidationError(null);

    if (!effectiveToken) {
      setValidationError("A valid reset token is required.");
      toast.error("Please provide your reset token from the email link.", "TOKEN REQUIRED");
      return;
    }

    if (password.length < 8) {
      setValidationError("Password must be at least 8 characters long.");
      toast.error("Password must be at least 8 characters long.", "VALIDATION ERROR");
      return;
    }

    if (password !== confirmPassword) {
      setValidationError("Passwords do not match.");
      toast.error("Password confirmation does not match.", "VALIDATION ERROR");
      return;
    }

    try {
      const response = await resetPasswordMutation.mutateAsync({
        token: effectiveToken,
        password,
        confirm_password: confirmPassword,
      });

      setIsSuccess(true);
      toast.success(
        response.message || "Password successfully changed. You can now log in.",
        "PASSWORD UPDATED"
      );
    } catch (err: any) {
      const errorMsg =
        err?.error || err?.message || "Failed to reset password. The link or token may be expired.";
      toast.error(errorMsg, "RESET FAILED");
      setValidationError(errorMsg);
    }
  };

  return (
    <div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center p-4 font-sans">
      <div className="w-full max-w-md">
        <div className="translucent-container p-2 sm:p-3">
          <Card className="border border-[var(--card-border)] shadow-none">
            <CardHeader className="pb-4">
              <div className="flex items-center justify-between">
                <CardTitle className="text-xl font-bold tracking-tight text-[var(--ink-primary)]">
                  Reset Password
                </CardTitle>
                <Badge variant="blue" className="text-[10px] tracking-wider uppercase font-semibold">
                  CREDENTIALS
                </Badge>
              </div>
              <CardDescription className="text-xs text-[var(--ink-secondary)] mt-1">
                Enter a new secure password for your Cadence account.
              </CardDescription>
            </CardHeader>

            {isSuccess ? (
              <div>
                <CardContent className="space-y-4">
                  <Alert variant="success" title="PASSWORD RESET SUCCESSFUL">
                    Your account password has been updated. You can now sign in with your new credentials.
                  </Alert>
                </CardContent>

                <CardFooter className="flex flex-col gap-2 pt-3">
                  <Link href="/login" className="w-full">
                    <Button variant="primary" className="w-full h-11 text-sm font-semibold">
                      Proceed to Login →
                    </Button>
                  </Link>
                </CardFooter>
              </div>
            ) : (
              <form onSubmit={handleSubmit}>
                <CardContent className="space-y-4">
                  {/* Token status banner */}
                  {hasUrlToken ? (
                    <div className="rounded-[14px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-3 text-xs flex items-center justify-between">
                      <div className="space-y-0.5">
                        <span className="text-[10px] uppercase font-bold text-[var(--ink-tertiary)]">
                          Security Verification
                        </span>
                        <p className="font-semibold text-[var(--ink-primary)]">Reset Token Attached</p>
                      </div>
                      <Badge variant="solid" size="sm">
                        VERIFIED
                      </Badge>
                    </div>
                  ) : (
                    <div className="space-y-2">
                      <Alert variant="warning" title="TOKEN REQUIRED">
                        No reset token detected in the URL. Please enter the token from your reset email below.
                      </Alert>
                      <Input
                        label="Security Token"
                        type="text"
                        required
                        placeholder="Paste 64-character token..."
                        value={manualToken}
                        onChange={(e) => setManualToken(e.target.value)}
                      />
                    </div>
                  )}

                  {validationError && (
                    <Alert variant="error" title="ERROR">
                      {validationError}
                    </Alert>
                  )}

                  <Input
                    label="New Password"
                    type="password"
                    required
                    autoComplete="new-password"
                    placeholder="Minimum 8 characters"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    helperText="Must be at least 8 characters long"
                  />

                  <Input
                    label="Confirm New Password"
                    type="password"
                    required
                    autoComplete="new-password"
                    placeholder="Re-type password"
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
                  />
                </CardContent>

                <CardFooter className="flex flex-col gap-3 pt-3">
                  <Button
                    type="submit"
                    variant="primary"
                    className="w-full h-11 text-sm font-semibold"
                    isLoading={resetPasswordMutation.isPending}
                  >
                    Set New Password
                  </Button>

                  <div className="flex w-full justify-center text-xs text-[var(--ink-secondary)] pt-2 border-t border-[var(--card-line)]">
                    <Link
                      href="/login"
                      className="hover:text-[var(--ink-primary)] underline transition-colors"
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
    </div>
  );
}

export default function ResetPasswordPage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center p-4 font-sans">
          <div className="w-full max-w-md">
            <SkeletonCard />
          </div>
        </div>
      }
    >
      <ResetPasswordForm />
    </Suspense>
  );
}
