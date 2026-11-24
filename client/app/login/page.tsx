"use client";

import React, { Suspense, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useAuth } from "@/providers/auth-provider";
import { authService } from "@/services/auth.service";
import { getTenantScope } from "@/lib/api-client";
import { getAuthenticatedHomeRoute } from "@/lib/auth-routes";
import { getOAuthErrorMessage } from "@/lib/oauth-callback";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { SkeletonCard } from "@/components/ui/skeleton";
import { Alert } from "@/components/ui/alert";

function GoogleIcon() {
  return (
    <svg
      className="h-4 w-4 shrink-0"
      viewBox="0 0 24 24"
      aria-hidden="true"
    >
      <path
        fill="#4285F4"
        d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"
      />
      <path
        fill="#34A853"
        d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
      />
      <path
        fill="#FBBC05"
        d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"
      />
      <path
        fill="#EA4335"
        d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"
      />
    </svg>
  );
}

function LoginContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { user, isAuthenticated, isLoading } = useAuth();
  const [isRedirecting, setIsRedirecting] = useState(false);

  const errorCode = searchParams.get("error");
  const redirectParam = searchParams.get("redirect");
  const errorMessage = errorCode ? getOAuthErrorMessage(errorCode) : null;

  useEffect(() => {
    if (!isLoading && isAuthenticated && user) {
      const { eventSlug } = getTenantScope();
      router.replace(getAuthenticatedHomeRoute(user, eventSlug));
    }
  }, [isLoading, isAuthenticated, user, router]);

  const handleGoogleSignIn = () => {
    setIsRedirecting(true);
    window.location.assign(authService.getGoogleLoginUrl(redirectParam));
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
        <Card>
          <CardHeader className="pb-4">
            <CardTitle className="text-xl font-bold tracking-tight text-[var(--ink-primary)]">
              Sign in
            </CardTitle>
            <CardDescription className="text-xs text-[var(--ink-secondary)] mt-1">
              Continue with your Google account to access Cadence.
            </CardDescription>
            <p className="text-xs text-[var(--ink-secondary)] mt-3 rounded-[10px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-2.5">
              Sign in with the email address registered by your event organizer. After signing in, you&apos;ll select which event to enter.
            </p>
          </CardHeader>

          <CardContent className="space-y-4">
            {errorMessage && (
              <Alert variant="error" title="Sign in failed">
                {errorMessage}
              </Alert>
            )}
          </CardContent>

          <CardFooter className="flex flex-col gap-3 pt-1">
            <Button
              type="button"
              variant="primary"
              className="w-full h-11 text-sm font-semibold gap-2.5"
              isLoading={isRedirecting}
              onClick={handleGoogleSignIn}
            >
              {!isRedirecting && <GoogleIcon />}
              <span>Continue with Google</span>
            </Button>
          </CardFooter>
        </Card>
      </div>
    </div>
  );
}

export default function LoginPage() {
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
      <LoginContent />
    </Suspense>
  );
}
