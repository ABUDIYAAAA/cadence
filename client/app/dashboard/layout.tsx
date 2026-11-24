"use client";

import React, { useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";
import { useAuth } from "@/providers/auth-provider";
import { useTenant } from "@/providers/tenant-provider";
import { Sidebar } from "@/components/layout/sidebar";
import { SkeletonCard } from "@/components/ui/skeleton";

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const router = useRouter();
  const pathname = usePathname();
  const { isAuthenticated, isDev, isLoading } = useAuth();
  const {
    eventSlug,
    competitionSlug,
    competitions,
    isLoadingCompetitions,
  } = useTenant();

  const canAccessWithoutEvent =
    isDev &&
    (pathname === "/dashboard/dev" ||
      pathname === "/dashboard/users" ||
      pathname === "/dashboard/audit");

  useEffect(() => {
    if (isLoading) return;
    if (!isAuthenticated) {
      router.replace("/login");
    } else if (!eventSlug && !canAccessWithoutEvent) {
      router.replace("/events");
    }
  }, [isLoading, isAuthenticated, eventSlug, canAccessWithoutEvent, router]);

  const waitingForCompetitionSelection =
    Boolean(eventSlug) &&
    (isLoadingCompetitions || (competitions.length > 0 && !competitionSlug));

  const isBlocked =
    isLoading ||
    (isAuthenticated &&
      ((!eventSlug && !canAccessWithoutEvent) || waitingForCompetitionSelection));

  return (
    <div className="flex min-h-[calc(100vh-3.5rem)] w-full font-sans bg-[var(--background)]">
      {!isBlocked && <Sidebar />}
      <div className="flex-1 min-w-0">
        {isBlocked && (
          <div className="flex min-h-[calc(100vh-3.5rem)] w-full items-center justify-center p-6">
            <div className="w-full max-w-xl">
              <SkeletonCard />
            </div>
          </div>
        )}
        <div
          className={
            isBlocked
              ? "hidden"
              : "w-full px-5 sm:px-8 lg:px-10 py-6 sm:py-8"
          }
        >
          {children}
        </div>
      </div>
    </div>
  );
}
