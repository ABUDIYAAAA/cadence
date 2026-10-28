"use client";

import React, { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/providers/auth-provider";
import { UserRole } from "@/types/auth";
import { getDashboardRoute } from "@/lib/auth-routes";
import { Card, CardContent, CardFooter, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { SkeletonCard } from "@/components/ui/skeleton";
import { Alert } from "@/components/ui/alert";

interface RoleDashboardProps {
  expectedRole: UserRole;
  title: string;
  description: string;
  children?: React.ReactNode;
}

export function RoleDashboard({
  expectedRole,
  title,
  description,
  children,
}: RoleDashboardProps) {
  const router = useRouter();
  const { user, isAuthenticated, isLoading, logout } = useAuth();
  const [isLoggingOut, setIsLoggingOut] = useState(false);

  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      router.replace("/login");
    }
  }, [isLoading, isAuthenticated, router]);

  const handleLogout = async () => {
    setIsLoggingOut(true);
    try {
      await logout();
    } finally {
      setIsLoggingOut(false);
    }
  };

  if (isLoading) {
    return (
      <div className="mx-auto max-w-4xl p-6 space-y-6">
        <SkeletonCard />
      </div>
    );
  }

  if (!isAuthenticated || !user) {
    return (
      <div className="mx-auto max-w-4xl p-6">
        <SkeletonCard />
      </div>
    );
  }

  const isRoleMismatch = user.role !== expectedRole;

  return (
    <div className="mx-auto max-w-5xl space-y-6 font-sans">
      {isRoleMismatch && (
        <Alert variant="warning" title="ROLE MISMATCH DETECTED">
          Your account role is{" "}
          <strong className="uppercase">{user.role}</strong>, but you are viewing
          the <strong className="uppercase">{expectedRole}</strong> dashboard.
          <div className="mt-3">
            <Button
              variant="outline"
              size="sm"
              onClick={() => router.push(getDashboardRoute(user.role))}
            >
              Go to Your Dashboard ({user.role})
            </Button>
          </div>
        </Alert>
      )}

      {/* Main Glass Container (Layer 1) */}
      <div className="translucent-container">
        {/* Container Header */}
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-6 pb-4 border-b border-[var(--glass-container-border)]">
          <div>
            <div className="flex items-center gap-2 mb-1.5">
              <span className="pill pill-blue">
                {expectedRole} Console
              </span>
              <span className="text-xs text-[var(--ink-secondary)] font-semibold">
                Live Session
              </span>
            </div>
            <h1 className="text-xl sm:text-2xl font-extrabold tracking-tight text-[var(--ink-primary)]">
              {title}
            </h1>
            <p className="text-xs text-[var(--ink-secondary)] mt-1">
              {description}
            </p>
          </div>

          <Badge variant="solid" size="md">
            {user.role}
          </Badge>
        </div>

        {/* Content Card (Layer 2) */}
        <div className="content-card-solid">
          <div className="rounded-[16px] border border-[var(--card-line)] divide-y divide-[var(--card-line)] overflow-hidden">
            <div className="flex flex-col sm:flex-row justify-between p-3.5 text-xs bg-[var(--card-solid-subtle)]">
              <span className="text-[var(--ink-secondary)] font-medium">Full Name</span>
              <span className="font-bold text-[var(--ink-primary)]">{user.name}</span>
            </div>

            <div className="flex flex-col sm:flex-row justify-between p-3.5 text-xs bg-[var(--card-solid)]">
              <span className="text-[var(--ink-secondary)] font-medium">Email Address</span>
              <span className="font-mono text-[var(--ink-primary)]">{user.email}</span>
            </div>

            <div className="flex flex-col sm:flex-row justify-between p-3.5 text-xs bg-[var(--card-solid-subtle)]">
              <span className="text-[var(--ink-secondary)] font-medium">User Identifier (UUID)</span>
              <span className="font-mono text-[var(--ink-secondary)] break-all">{user.id}</span>
            </div>

            <div className="flex flex-col sm:flex-row justify-between p-3.5 text-xs bg-[var(--card-solid)]">
              <span className="text-[var(--ink-secondary)] font-medium">System Role</span>
              <span className="uppercase font-semibold text-[var(--ink-primary)]">{user.role}</span>
            </div>

            <div className="flex flex-col sm:flex-row justify-between p-3.5 text-xs bg-[var(--card-solid-subtle)]">
              <span className="text-[var(--ink-secondary)] font-medium">Phone Number</span>
              <span className="font-mono text-[var(--ink-primary)]">
                {user.phone_number || "— (Not Provided)"}
              </span>
            </div>

            <div className="flex flex-col sm:flex-row justify-between p-3.5 text-xs bg-[var(--card-solid)]">
              <span className="text-[var(--ink-secondary)] font-medium">Assigned Team</span>
              <span className="font-mono text-[var(--ink-primary)]">
                {user.team_id ? user.team_id : "— (Unassigned)"}
              </span>
            </div>

            <div className="flex flex-col sm:flex-row justify-between p-3.5 text-xs bg-[var(--card-solid-subtle)]">
              <span className="text-[var(--ink-secondary)] font-medium">Password Setup</span>
              <span className="font-semibold text-[var(--ink-primary)]">
                {user.is_password_configured ? "Configured" : "Pending Setup"}
              </span>
            </div>

            <div className="flex flex-col sm:flex-row justify-between p-3.5 text-xs bg-[var(--card-solid)]">
              <span className="text-[var(--ink-secondary)] font-medium">Account Created</span>
              <span className="font-mono text-[var(--ink-secondary)]">
                {new Date(user.created_at).toLocaleString()}
              </span>
            </div>
          </div>

          <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 pt-5 mt-5 border-t border-[var(--card-line)]">
            <p className="text-xs text-[var(--ink-secondary)]">
              Session authenticated with signed HTTP-only cookies.
            </p>

            <Button
              variant="destructive"
              size="md"
              isLoading={isLoggingOut}
              onClick={handleLogout}
              className="w-full sm:w-auto"
            >
              Log Out
            </Button>
          </div>
        </div>
      </div>

      {children}
    </div>
  );
}
