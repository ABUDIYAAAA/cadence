"use client";

import React from "react";
import { usePathname } from "next/navigation";
import { useAuth } from "@/providers/auth-provider";
import { useTenant } from "@/providers/tenant-provider";
import { Sidebar } from "@/components/layout/sidebar";

export function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const { isAuthenticated, isDev, isLoading } = useAuth();
  const { eventSlug } = useTenant();

  const isDashboard = pathname.startsWith("/dashboard");
  const isEvents = pathname === "/events";

  const canAccessDashboardWithoutEvent =
    isDev &&
    (pathname === "/dashboard/dev" ||
      pathname === "/dashboard/users" ||
      pathname === "/dashboard/audit");

  // Keep one persistent Sidebar mounted across /dashboard/* and /events so hover and active-tab transitions stay smooth.
  const showSidebar =
    !isLoading &&
    isAuthenticated &&
    ((isDashboard && (Boolean(eventSlug) || canAccessDashboardWithoutEvent)) ||
      (isEvents && (isDev || Boolean(eventSlug))));

  if (!showSidebar) {
    return <main className="flex-1">{children}</main>;
  }

  return (
    <main className="flex-1 flex min-h-[calc(100vh-3.5rem)] w-full font-sans bg-[var(--background)]">
      <Sidebar />
      <div className="flex-1 min-w-0">{children}</div>
    </main>
  );
}
