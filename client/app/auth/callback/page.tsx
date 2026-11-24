"use client";

import React, { Suspense, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useQueryClient } from "@tanstack/react-query";
import { AUTH_QUERY_KEY } from "@/hooks/use-auth";
import { authService } from "@/services/auth.service";
import { clearTenantScope } from "@/lib/api-client";
import { parseOAuthCallbackParams } from "@/lib/oauth-callback";
import { useToast } from "@/providers/toast-provider";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Spinner } from "@/components/ui/spinner";

function OAuthCallbackContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const queryClient = useQueryClient();
  const toast = useToast();

  const parsed = useMemo(
    () => parseOAuthCallbackParams(searchParams),
    [searchParams]
  );

  const [verifyError, setVerifyError] = useState<string | null>(null);
  const [isRetrying, setIsRetrying] = useState(false);

  useEffect(() => {
    if (parsed.status !== "success") return;

    const targetRedirect = parsed.redirectPath;
    let cancelled = false;

    async function completeSignIn() {
      try {
        // Clear any previous event selection so the user is prompted to choose their Event
        clearTenantScope();

        const user = await queryClient.fetchQuery({
          queryKey: AUTH_QUERY_KEY,
          queryFn: () => authService.me(),
          staleTime: 0,
        });

        if (cancelled) return;

        toast.success(`Welcome, ${user.name}`, "AUTHENTICATED");
        router.replace(targetRedirect);
      } catch {
        if (cancelled) return;
        setVerifyError(
          "We could not verify your active session cookie. Please try signing in again."
        );
      }
    }

    completeSignIn();

    return () => {
      cancelled = true;
    };
  }, [parsed, queryClient, router, toast]);

  const handleRetryGoogle = () => {
    setIsRetrying(true);
    window.location.assign(authService.getGoogleLoginUrl());
  };

  if (parsed.status === "error" || verifyError) {
    const errorTitle =
      parsed.status === "error" ? "Google sign-in failed" : "Session verification failed";
    const errorMessage =
      parsed.status === "error" ? parsed.message : verifyError;
    const errorCode =
      parsed.status === "error" ? parsed.code : "session_verification_failed";

    return (
      <div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center p-4 font-sans">
        <div className="w-full max-w-md">
          <Card>
            <CardHeader className="pb-3">
              <div className="flex items-center justify-between gap-2">
                <CardTitle className="text-xl font-bold tracking-tight text-[var(--ink-primary)]">
                  Unable to sign in
                </CardTitle>
                <Badge variant="subtle" size="sm">
                  {errorCode}
                </Badge>
              </div>
              <CardDescription className="text-xs text-[var(--ink-secondary)] mt-1">
                We couldn&apos;t complete Google authentication for this account.
              </CardDescription>
            </CardHeader>

            <CardContent>
              <Alert variant="error" title={errorTitle}>
                {errorMessage}
              </Alert>
            </CardContent>

            <CardFooter className="flex flex-col sm:flex-row gap-2 pt-3">
              <Button
                type="button"
                variant="primary"
                className="w-full sm:flex-1 h-10 text-xs font-semibold"
                isLoading={isRetrying}
                onClick={handleRetryGoogle}
              >
                Try again with Google
              </Button>
              <Link href="/login" className="w-full sm:w-auto">
                <Button
                  type="button"
                  variant="outline"
                  className="w-full h-10 text-xs"
                  disabled={isRetrying}
                >
                  Back to sign in
                </Button>
              </Link>
            </CardFooter>
          </Card>
        </div>
      </div>
    );
  }

  const eventCount = parsed.events.length;

  return (
    <div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center p-4 font-sans">
      <div className="w-full max-w-md">
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-10 px-6 text-center space-y-4">
            <Spinner size="lg" />
            <div className="space-y-1">
              <h1 className="text-base font-bold tracking-tight text-[var(--ink-primary)]">
                Completing Google sign-in...
              </h1>
              <p className="text-xs text-[var(--ink-secondary)]">
                {eventCount > 0
                  ? `Found ${eventCount} ${eventCount === 1 ? "event" : "events"} linked to your email. Redirecting to event selection...`
                  : "Verifying your session and loading available events..."}
              </p>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

export default function OAuthCallbackPage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center p-4 font-sans">
          <Spinner size="lg" />
        </div>
      }
    >
      <OAuthCallbackContent />
    </Suspense>
  );
}
