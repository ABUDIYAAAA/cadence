"use client";

import React, { useEffect } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { QrCode, Users } from "lucide-react";
import { useAuth } from "@/providers/auth-provider";
import { getDashboardRoute } from "@/lib/auth-routes";
import { useVolunteerSchedule } from "@/hooks/use-judging";
import { useTicketAnalytics } from "@/hooks/use-tickets";
import { Skeleton, SkeletonCard } from "@/components/ui/skeleton";
import { MetricGrid, Metric } from "@/components/judging/section-shell";
import { VolunteerScheduleView } from "@/components/judging";

export default function VolunteerDashboardPage() {
  const router = useRouter();
  const { user, isAuthenticated, isLoading } = useAuth();
  const isVolunteer = user?.role === "volunteer" || user?.role === "admin";

  useEffect(() => {
    if (isLoading) return;
    if (!isAuthenticated || !user) {
      router.replace("/login");
    } else if (user.role !== "volunteer" && user.role !== "admin") {
      router.replace(getDashboardRoute(user.role));
    }
  }, [isLoading, isAuthenticated, user, router]);

  const { data: rawSchedule, isLoading: scheduleLoading } = useVolunteerSchedule();
  const schedule = Array.isArray(rawSchedule) ? rawSchedule : [];

  const { data: analyticsData, isLoading: analyticsLoading } = useTicketAnalytics();

  if (isLoading || !isVolunteer) {
    return <SkeletonCard />;
  }

  const firstName = user?.name?.split(" ")[0] || "Volunteer";

  // Compute live staging numbers
  const totalRooms = schedule.length;
  let activeRooms = 0;
  let onDeckCount = 0;
  let totalEvaluated = 0;
  let totalPitches = 0;

  schedule.forEach((panel) => {
    if (panel.active_team) activeRooms++;
    onDeckCount += (panel.queued_teams || []).length;
    totalEvaluated += panel.completed_count || 0;
    totalPitches += panel.total_count || 0;
  });

  const checkInRate = analyticsData
    ? Math.round((analyticsData.attendance_rate > 1 ? analyticsData.attendance_rate : analyticsData.attendance_rate * 100))
    : 0;

  return (
    <div className="font-sans space-y-6">
      {/* Header */}
      <header className="flex flex-col sm:flex-row sm:items-end justify-between gap-3">
        <div>
          <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-[var(--ink-primary)]">
            Welcome, {firstName}
          </h1>
          <p className="text-sm text-[var(--ink-secondary)] mt-1">
            Room queues and gate check-in.
          </p>
        </div>

        <div className="flex items-center gap-2">
          <Link
            href="/dashboard/tickets"
            className="inline-flex h-9 items-center gap-1.5 rounded-[10px] bg-[var(--color-dark)] px-4 text-xs font-semibold text-[var(--ink-inverse)] transition-transform duration-150 active:scale-[0.97]"
          >
            <QrCode className="h-3.5 w-3.5" />
            <span>Gate scanner</span>
          </Link>
          <Link
            href="/dashboard/teams"
            className="inline-flex h-9 items-center gap-1.5 rounded-[10px] border border-[var(--card-line)] bg-[var(--card-solid)] px-3 text-xs font-semibold text-[var(--ink-primary)] transition-transform duration-150 hover:bg-[var(--card-solid-subtle)] active:scale-[0.97]"
          >
            <Users className="h-3.5 w-3.5 text-[var(--ink-secondary)]" />
            <span>Teams</span>
          </Link>
        </div>
      </header>

      {/* Primary Metrics Row */}
      <MetricGrid>
        <Metric
          label="Active rooms"
          value={scheduleLoading ? <Skeleton className="h-7 w-12" /> : activeRooms}
          suffix={totalRooms > 0 ? `/ ${totalRooms}` : undefined}
        />
        <Metric
          label="Teams on-deck"
          value={scheduleLoading ? <Skeleton className="h-7 w-12" /> : onDeckCount}
          valueClassName="text-[var(--color-coral)]"
        />
        <Metric
          label="Pitches completed"
          value={scheduleLoading ? <Skeleton className="h-7 w-12" /> : totalEvaluated}
          suffix={totalPitches > 0 ? `/ ${totalPitches}` : undefined}
          valueClassName="text-[var(--color-blue)]"
        />
        <Metric
          label="Gate check-ins"
          value={analyticsLoading ? <Skeleton className="h-7 w-14" /> : analyticsData?.checked_in_count ?? 0}
          suffix={analyticsData ? `/ ${analyticsData.total_issued}` : undefined}
          note={`${checkInRate}% checked in`}
        />
      </MetricGrid>

      {/* Embedded Live Room Staging Monitor */}
      <section>
        <VolunteerScheduleView showStats={false} />
      </section>
    </div>
  );
}
