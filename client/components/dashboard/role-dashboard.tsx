"use client";

import React, { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/providers/auth-provider";
import { UserRole } from "@/types/auth";
import { getDashboardRoute } from "@/lib/auth-routes";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
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
    <div className="mx-auto max-w-4xl px-4 py-8 sm:px-6 space-y-6 font-mono">
      {isRoleMismatch && (
        <Alert variant="warning" title="ROLE MISMATCH DETECTED">
          Your account role is{" "}
          <strong className="uppercase">{user.role}</strong>, but you are viewing
          the <strong className="uppercase">{expectedRole}</strong> dashboard.
          <div className="mt-2">
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

      <Card>
        <CardHeader>
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
            <div>
              <div className="flex items-center gap-2 mb-1">
                <span className="text-[10px] bg-black text-white px-2 py-0.5 uppercase tracking-wider font-bold">
                  {expectedRole} CONSOLE
                </span>
                <span className="text-xs text-neutral-500">LIVE SESSION</span>
              </div>
              <CardTitle>{title}</CardTitle>
              <CardDescription>{description}</CardDescription>
            </div>

            <Badge variant="solid" size="md">
              {user.role}
            </Badge>
          </div>
        </CardHeader>

        <CardContent className="space-y-4">
          <div className="border border-neutral-300 divide-y divide-neutral-200">
            <div className="flex flex-col sm:flex-row justify-between p-3.5 text-xs bg-neutral-50">
              <span className="text-neutral-600 font-medium">Full Name</span>
              <span className="font-bold text-black">{user.name}</span>
            </div>

            <div className="flex flex-col sm:flex-row justify-between p-3.5 text-xs">
              <span className="text-neutral-600 font-medium">Email Address</span>
              <span className="font-mono text-black">{user.email}</span>
            </div>

            <div className="flex flex-col sm:flex-row justify-between p-3.5 text-xs bg-neutral-50">
              <span className="text-neutral-600 font-medium">User Identifier (UUID)</span>
              <span className="font-mono text-neutral-800 break-all">{user.id}</span>
            </div>

            <div className="flex flex-col sm:flex-row justify-between p-3.5 text-xs">
              <span className="text-neutral-600 font-medium">System Role</span>
              <span className="uppercase font-semibold text-black">{user.role}</span>
            </div>

            <div className="flex flex-col sm:flex-row justify-between p-3.5 text-xs bg-neutral-50">
              <span className="text-neutral-600 font-medium">Phone Number</span>
              <span className="font-mono text-black">
                {user.phone_number || "— (Not Provided)"}
              </span>
            </div>

            <div className="flex flex-col sm:flex-row justify-between p-3.5 text-xs">
              <span className="text-neutral-600 font-medium">Assigned Team</span>
              <span className="font-mono text-black">
                {user.team_id ? user.team_id : "— (Unassigned)"}
              </span>
            </div>

            <div className="flex flex-col sm:flex-row justify-between p-3.5 text-xs bg-neutral-50">
              <span className="text-neutral-600 font-medium">Security Password Setup</span>
              <span className="font-mono text-black">
                {user.is_password_configured ? "Configured" : "Pending Setup"}
              </span>
            </div>

            <div className="flex flex-col sm:flex-row justify-between p-3.5 text-xs">
              <span className="text-neutral-600 font-medium">Account Created</span>
              <span className="font-mono text-neutral-700">
                {new Date(user.created_at).toLocaleString()}
              </span>
            </div>
          </div>
        </CardContent>

        <CardFooter className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
          <p className="text-xs text-neutral-500">
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
        </CardFooter>
      </Card>

      {children}
    </div>
  );
}
