"use client";

import React, { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/providers/auth-provider";
import { UserRole } from "@/types/auth";
import { getDashboardRoute } from "@/lib/auth-routes";
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
      <div className="space-y-6">
        <SkeletonCard />
      </div>
    );
  }

  if (!isAuthenticated || !user) {
    return (
      <div className="">
        <SkeletonCard />
      </div>
    );
  }

  const isRoleMismatch = user.role !== expectedRole;

  return (
    <div className="space-y-6 font-sans">
      {isRoleMismatch && (
        <Alert variant="warning" title="Role mismatch">
          Your account role is{" "}
          <strong className="capitalize">{user.role}</strong>, but you are viewing
          the <strong className="capitalize">{expectedRole}</strong> dashboard.
          <div className="mt-3">
            <Button
              variant="outline"
              size="sm"
              onClick={() => router.push(getDashboardRoute(user.role))}
            >
              Go to your dashboard ({user.role})
            </Button>
          </div>
        </Alert>
      )}

      {/* Main Surface Panel */}
      <div className="space-y-6">
        {/* Header */}
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-4 border-b border-[var(--card-line)]">
          <div>
            <div className="flex items-center gap-2 mb-1.5">
              <Badge variant="blue" size="sm" className="capitalize">
                {expectedRole} portal
              </Badge>
              <span className="text-xs text-[var(--ink-secondary)] font-medium">
                Signed in
              </span>
            </div>
            <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-[var(--ink-primary)]">
              {title}
            </h1>
            <p className="text-xs text-[var(--ink-secondary)] mt-1">
              {description}
            </p>
          </div>

          <Badge variant="subtle" size="sm" className="capitalize">
            {user.role}
          </Badge>
        </div>

        {/* User Account Details Card */}
        <div className="rounded-[10px] border border-[var(--card-line)] divide-y divide-[var(--card-line)] overflow-hidden bg-[var(--card-solid)]">
          <div className="flex flex-col sm:flex-row justify-between p-3 text-xs bg-[var(--card-solid-subtle)]">
            <span className="text-[var(--ink-secondary)] font-medium">Name</span>
            <span className="font-semibold text-[var(--ink-primary)]">{user.name}</span>
          </div>

          <div className="flex flex-col sm:flex-row justify-between p-3 text-xs bg-[var(--card-solid)]">
            <span className="text-[var(--ink-secondary)] font-medium">Email</span>
            <span className="font-mono text-[var(--ink-primary)]">{user.email}</span>
          </div>

          <div className="flex flex-col sm:flex-row justify-between p-3 text-xs bg-[var(--card-solid-subtle)]">
            <span className="text-[var(--ink-secondary)] font-medium">User ID</span>
            <span className="font-mono text-[var(--ink-secondary)] break-all">{user.id}</span>
          </div>

          <div className="flex flex-col sm:flex-row justify-between p-3 text-xs bg-[var(--card-solid)]">
            <span className="text-[var(--ink-secondary)] font-medium">Role</span>
            <span className="capitalize font-medium text-[var(--ink-primary)]">{user.role}</span>
          </div>

          <div className="flex flex-col sm:flex-row justify-between p-3 text-xs bg-[var(--card-solid-subtle)]">
            <span className="text-[var(--ink-secondary)] font-medium">Phone</span>
            <span className="font-mono text-[var(--ink-primary)]">
              {user.phone_number || "—"}
            </span>
          </div>

          <div className="flex flex-col sm:flex-row justify-between p-3 text-xs bg-[var(--card-solid)]">
            <span className="text-[var(--ink-secondary)] font-medium">Team</span>
            <span className="font-mono text-[var(--ink-primary)]">
              {user.team_id ? user.team_id : "Unassigned"}
            </span>
          </div>

          <div className="flex flex-col sm:flex-row justify-between p-3 text-xs bg-[var(--card-solid-subtle)]">
            <span className="text-[var(--ink-secondary)] font-medium">Password</span>
            <span className="font-medium text-[var(--ink-primary)]">
              {user.is_password_configured ? "Configured" : "Pending setup"}
            </span>
          </div>

          <div className="flex flex-col sm:flex-row justify-between p-3 text-xs bg-[var(--card-solid)]">
            <span className="text-[var(--ink-secondary)] font-medium">Created</span>
            <span className="font-mono text-[var(--ink-secondary)]">
              {new Date(user.created_at).toLocaleString()}
            </span>
          </div>
        </div>

        <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 pt-3 border-t border-[var(--card-line)]">
          <p className="text-xs text-[var(--ink-secondary)]">
            Authenticated via secure HTTP-only session cookies.
          </p>

          <Button
            variant="destructive"
            size="sm"
            isLoading={isLoggingOut}
            onClick={handleLogout}
            className="w-full sm:w-auto"
          >
            Sign out
          </Button>
        </div>
      </div>

      {children}
    </div>
  );
}
