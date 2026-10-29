"use client";

import React, { useEffect } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  QrCode,
  DoorOpen,
  Layers,
  ArrowRight,
  Volume2,
  CheckCircle2,
  Clock,
  ExternalLink,
} from "lucide-react";
import { useAuth } from "@/providers/auth-provider";
import { getDashboardRoute } from "@/lib/auth-routes";
import { useVolunteerSchedule } from "@/hooks/use-judging";
import { useTicketAnalytics } from "@/hooks/use-tickets";
import { Skeleton, SkeletonCard } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
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
            Coordinate room staging, manage on-deck queues, and handle gate check-in scanning.
          </p>
        </div>

        <div className="flex items-center gap-2">
          <Link
            href="/dashboard/tickets"
            className="inline-flex h-9 items-center gap-1.5 rounded-[10px] bg-[var(--color-dark)] px-4 text-xs font-semibold text-[var(--ink-inverse)] transition-transform duration-150 active:scale-[0.97]"
          >
            <QrCode className="h-3.5 w-3.5" />
            <span>Open Gate Scanner</span>
          </Link>
          <Link
            href="/dashboard/judging"
            className="inline-flex h-9 items-center gap-1.5 rounded-[10px] border border-[var(--card-line)] bg-[var(--card-solid)] px-3 text-xs font-semibold text-[var(--ink-primary)] transition-transform duration-150 hover:bg-[var(--card-solid-subtle)] active:scale-[0.97]"
          >
            <DoorOpen className="h-3.5 w-3.5 text-[var(--ink-secondary)]" />
            <span>Staging Fullview</span>
          </Link>
        </div>
      </header>

      {/* Primary Metrics Row */}
      <MetricGrid>
        <Metric
          label="Active rooms"
          value={scheduleLoading ? <Skeleton className="h-7 w-12" /> : activeRooms}
          suffix={totalRooms > 0 ? `/ ${totalRooms}` : undefined}
          note={totalRooms > 0 ? `${totalRooms} rooms configured` : "No rooms active"}
        />
        <Metric
          label="Teams on-deck"
          value={scheduleLoading ? <Skeleton className="h-7 w-12" /> : onDeckCount}
          valueClassName="text-[var(--color-coral)]"
          note="Ready to escort"
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

      {/* Quick Action Navigation Surface */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {/* Gate Scanner Tile */}
        <div className="rounded-[16px] border border-[var(--card-line)] bg-[var(--card-solid)] p-5 flex flex-col justify-between space-y-4 hover:border-[var(--color-blue-border)] transition-colors duration-150">
          <div className="space-y-1.5">
            <div className="flex items-center justify-between">
              <span className="font-semibold text-sm text-[var(--ink-primary)]">
                Gate check-in & badges
              </span>
              <Badge variant="emerald" dot size="sm">
                Active
              </Badge>
            </div>
            <p className="text-xs text-[var(--ink-secondary)] leading-relaxed">
              Verify attendee admission QR codes via camera scanner with duplicate check detection and manual attendee lookup.
            </p>
          </div>
          <div className="pt-2">
            <Link
              href="/dashboard/tickets"
              className="inline-flex h-9 w-full items-center justify-center gap-1.5 rounded-[10px] bg-[var(--card-solid-subtle)] border border-[var(--card-line)] text-xs font-semibold text-[var(--ink-primary)] hover:bg-[var(--card-solid)] transition-all active:scale-[0.98]"
            >
              <span>Launch scanner</span>
              <ArrowRight className="h-3.5 w-3.5" />
            </Link>
          </div>
        </div>

        {/* Teams Directory Tile */}
        <div className="rounded-[16px] border border-[var(--card-line)] bg-[var(--card-solid)] p-5 flex flex-col justify-between space-y-4 hover:border-[var(--color-blue-border)] transition-colors duration-150">
          <div className="space-y-1.5">
            <div className="flex items-center justify-between">
              <span className="font-semibold text-sm text-[var(--ink-primary)]">
                Teams directory & rosters
              </span>
              <Badge variant="subtle" size="sm">
                Directory
              </Badge>
            </div>
            <p className="text-xs text-[var(--ink-secondary)] leading-relaxed">
              Search registered teams, verify member check-in states, and find team lead contact information.
            </p>
          </div>
          <div className="pt-2">
            <Link
              href="/dashboard/teams"
              className="inline-flex h-9 w-full items-center justify-center gap-1.5 rounded-[10px] bg-[var(--card-solid-subtle)] border border-[var(--card-line)] text-xs font-semibold text-[var(--ink-primary)] hover:bg-[var(--card-solid)] transition-all active:scale-[0.98]"
            >
              <span>View teams</span>
              <ArrowRight className="h-3.5 w-3.5" />
            </Link>
          </div>
        </div>
      </div>

      {/* Embedded Live Room Staging Monitor */}
      <section className="space-y-3 pt-2">
        <VolunteerScheduleView />
      </section>
    </div>
  );
}
