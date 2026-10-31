"use client";

import React, { useEffect } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { useAuth } from "@/providers/auth-provider";
import { getDashboardRoute } from "@/lib/auth-routes";
import { SkeletonCard } from "@/components/ui/skeleton";
import { MyTicketView } from "@/components/tickets/my-ticket-view";
import { MyTeamView } from "@/components/teams/my-team-view";
import { ParticipantJudgingStatusView } from "@/components/judging";

export default function ParticipantDashboardPage() {
  const router = useRouter();
  const { user, isAuthenticated, isLoading } = useAuth();

  useEffect(() => {
    if (isLoading) return;
    if (!isAuthenticated || !user) router.replace("/login");
    else if (user.role !== "participant") router.replace(getDashboardRoute(user.role));
  }, [isLoading, isAuthenticated, user, router]);

  if (isLoading || !user || user.role !== "participant") {
    return (
      <div className="space-y-4">
        <SkeletonCard />
      </div>
    );
  }

  const firstName = user.name?.split(" ")[0] || "there";

  return (
    <div className="font-sans space-y-6">
      <header>
        <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-[var(--ink-primary)] text-balance">
          Hi, {firstName}
        </h1>
        <p className="text-sm text-[var(--ink-secondary)] mt-1">
          Here&apos;s what you need for the event.
        </p>
      </header>

      <div className="grid grid-cols-1 lg:grid-cols-[minmax(0,1fr)_340px] gap-6 items-start">
        {/* Primary: my team (create/join/manage) and what's happening with it right now */}
        <div className="space-y-8 min-w-0">
          <ParticipantJudgingStatusView compact />
          <MyTeamView />
        </div>

        {/* Secondary: entry pass, needed at the door */}
        <aside className="space-y-3 lg:sticky lg:top-6">
          <div className="flex items-baseline justify-between">
            <h2 className="text-base font-semibold tracking-tight text-[var(--ink-primary)]">
              Entry pass
            </h2>
            <Link
              href="/dashboard/tickets"
              className="text-xs font-medium text-[var(--color-blue)] hover:underline"
            >
              Full screen
            </Link>
          </div>
          <MyTicketView compact />
        </aside>
      </div>
    </div>
  );
}
