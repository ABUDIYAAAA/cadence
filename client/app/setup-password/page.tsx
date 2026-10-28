"use client";

import React, { Suspense, useState } from "react";
import { useSearchParams } from "next/navigation";
import Link from "next/link";
import { useSetupPassword } from "@/hooks/use-auth";
import { useToast } from "@/providers/toast-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { SkeletonCard } from "@/components/ui/skeleton";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { ApiError } from "@/types/api";

function SetupPasswordForm() {
  const searchParams = useSearchParams();
  const urlToken = searchParams.get("token") || "";

  const setupPasswordMutation = useSetupPassword();
  const toast = useToast();

  const [manualToken, setManualToken] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [validationError, setValidationError] = useState<string | null>(null);
  const [isSuccess, setIsSuccess] = useState(false);

  const effectiveToken = urlToken.trim() || manualToken.trim();
  const hasUrlToken = Boolean(urlToken.trim());

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setValidationError(null);

    if (!effectiveToken) {
      setValidationError("Invitation token is required.");
      toast.error("Please enter your invitation setup token.", "TOKEN REQUIRED");
      return;
    }

    if (password.length < 8) {
      setValidationError("Password must be at least 8 characters long.");
      toast.error("Password must be at least 8 characters long.", "VALIDATION ERROR");
      return;
    }

    if (password !== confirmPassword) {
      setValidationError("Passwords do not match.");
      toast.error("Passwords do not match.", "VALIDATION ERROR");
      return;
    }

    try {
      const response = await setupPasswordMutation.mutateAsync({
        token: effectiveToken,
        password,
        confirm_password: confirmPassword,
      });

      setIsSuccess(true);
      toast.success(
        response.message || "Account activated! You can now log in.",
        "ACCOUNT CONFIGURED"
      );
    } catch (err: unknown) {
      const apiErr = err as ApiError;
      const errorMsg =
        apiErr?.error ||
        (err instanceof Error ? err.message : "Failed to set up account. Token may be invalid or expired.");
      toast.error(errorMsg, "SETUP FAILED");
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
                  Configure Account
                </CardTitle>
                <Badge variant="blue" className="text-[10px] tracking-wider uppercase font-semibold">
                  ACTIVATION
                </Badge>
              </div>
              <CardDescription className="text-xs text-[var(--ink-secondary)] mt-1">
                Enter your invitation token and set your account password.
              </CardDescription>
            </CardHeader>

            {isSuccess ? (
              <div>
                <CardContent className="space-y-4">
                  <Alert variant="success" title="ACCOUNT ACTIVATED">
                    Your password has been configured successfully. You may now log
                    in to your Cadence portal.
                  </Alert>
                </CardContent>

                <CardFooter className="pt-3">
                  <Link href="/login" className="w-full">
                    <Button variant="primary" className="w-full h-11 text-sm font-semibold">
                      Proceed to Login
                    </Button>
                  </Link>
                </CardFooter>
              </div>
            ) : (
              <form onSubmit={handleSubmit}>
                <CardContent className="space-y-4">
                  {!hasUrlToken ? (
                    <div className="space-y-3">
                      <div className="flex items-center justify-between">
                        <span className="text-xs font-semibold text-[var(--ink-primary)]">
                          Invitation Setup Token
                        </span>
                        <Badge variant="solid" size="sm">
                          REQUIRED
                        </Badge>
                      </div>

                      <Alert variant="warning" title="INVITATION TOKEN">
                        No token was found in your URL. Please paste the invitation
                        token sent by your administrator.
                      </Alert>

                      <Input
                        label="Token"
                        required
                        placeholder="Paste setup token here"
                        value={manualToken}
                        onChange={(e) => setManualToken(e.target.value)}
                        error={!effectiveToken && validationError ? validationError : undefined}
                      />
                    </div>
                  ) : (
                    <div className="flex items-center justify-between p-3 rounded-[14px] bg-[var(--card-solid)] border border-[var(--card-line)] text-xs">
                      <span className="text-[var(--ink-secondary)]">Token Attached</span>
                      <span className="font-mono text-[var(--color-blue)] font-semibold truncate max-w-[200px]">
                        {urlToken.slice(0, 10)}...{urlToken.slice(-6)}
                      </span>
                    </div>
                  )}

                  {validationError && (
                    <Alert variant="error" title="ERROR">
                      {validationError}
                    </Alert>
                  )}

                  <Input
                    label="Choose Password"
                    type="password"
                    required
                    autoComplete="new-password"
                    placeholder="••••••••"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    helperText="Minimum 8 characters with at least one letter and number."
                  />

                  <Input
                    label="Confirm Password"
                    type="password"
                    required
                    autoComplete="new-password"
                    placeholder="••••••••"
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
                    error={
                      confirmPassword && password !== confirmPassword
                        ? "Passwords do not match"
                        : undefined
                    }
                  />
                </CardContent>

                <CardFooter className="flex flex-col gap-3 pt-3">
                  <Button
                    type="submit"
                    variant="primary"
                    className="w-full h-11 text-sm font-semibold"
                    isLoading={setupPasswordMutation.isPending}
                  >
                    Activate Account
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

export default function SetupPasswordPage() {
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
      <SetupPasswordForm />
    </Suspense>
  );
}
