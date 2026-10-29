"use client";

import React, { useEffect } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  Award,
  Layers,
  ArrowRight,
  Clock,
  CheckCircle2,
  Lock,
  DoorOpen,
  Play,
  ExternalLink,
} from "lucide-react";
import { useAuth } from "@/providers/auth-provider";
import { getDashboardRoute } from "@/lib/auth-routes";
import { useJudgePanels } from "@/hooks/use-judging";
import { useTeams } from "@/hooks/use-teams";
import { Skeleton, SkeletonCard } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { MetricGrid, Metric } from "@/components/judging/section-shell";

export default function JudgeDashboardPage() {
  const router = useRouter();
  const { user, isAuthenticated, isLoading } = useAuth();
  const isJudge = user?.role === "judge" || user?.role === "admin";

  useEffect(() => {
    if (isLoading) return;
    if (!isAuthenticated || !user) {
      router.replace("/login");
    } else if (user.role !== "judge" && user.role !== "admin") {
      router.replace(getDashboardRoute(user.role));
    }
  }, [isLoading, isAuthenticated, user, router]);

  const { data: rawPanels, isLoading: panelsLoading } = useJudgePanels();
  const panels = Array.isArray(rawPanels) ? rawPanels : [];

  const { data: teamsData, isLoading: teamsLoading } = useTeams({ page: 1, limit: 1 });

  if (isLoading || !isJudge) {
    return <SkeletonCard />;
  }

  const firstName = user?.name?.split(" ")[0] || "Judge";

  // Aggregate metrics
  const totalPanels = panels.length;
  const totalSlots = panels.reduce((acc, p) => acc + (p.total_slots || 0), 0);
  const completedSlots = panels.reduce((acc, p) => acc + (p.completed_slots || 0), 0);
  const percentComplete = totalSlots > 0 ? Math.round((completedSlots / totalSlots) * 100) : 0;

  // Find active or upcoming slot across panels
  let activeSlot: {
    panel: (typeof panels)[0];
    slot: (typeof panels)[0]["slots"][0];
  } | null = null;

  for (const panel of panels) {
    const presenting = panel.slots.find((s) => s.status === "in_progress");
    if (presenting) {
      activeSlot = { panel, slot: presenting };
      break;
    }
    if (!activeSlot) {
      const queued = panel.slots.find((s) => s.status === "queued" || s.status === "scheduled");
      if (queued) {
        activeSlot = { panel, slot: queued };
      }
    }
  }

  return (
    <div className="font-sans space-y-6">
      {/* Header */}
      <header className="flex flex-col sm:flex-row sm:items-end justify-between gap-3">
        <div>
          <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-[var(--ink-primary)]">
            Welcome, {firstName}
          </h1>
          <p className="text-sm text-[var(--ink-secondary)] mt-1">
            Evaluate pitches, grade rubrics, and inspect team code submissions.
          </p>
        </div>

        <div className="flex items-center gap-2">
          <Link
            href="/dashboard/judging"
            className="inline-flex h-9 items-center gap-1.5 rounded-[10px] bg-[var(--color-dark)] px-4 text-xs font-semibold text-[var(--ink-inverse)] transition-transform duration-150 active:scale-[0.97]"
          >
            <Award className="h-3.5 w-3.5" />
            <span>Open Scorecards</span>
          </Link>
          <Link
            href="/dashboard/teams"
            className="inline-flex h-9 items-center gap-1.5 rounded-[10px] border border-[var(--card-line)] bg-[var(--card-solid)] px-3 text-xs font-semibold text-[var(--ink-primary)] transition-transform duration-150 hover:bg-[var(--card-solid-subtle)] active:scale-[0.97]"
          >
            <Layers className="h-3.5 w-3.5 text-[var(--ink-secondary)]" />
            <span>My Teams</span>
          </Link>
        </div>
      </header>

      {/* Metrics Row */}
      <MetricGrid>
        <Metric
          label="Assigned Panels"
          value={panelsLoading ? <Skeleton className="h-7 w-12" /> : totalPanels}
          note={totalPanels === 1 ? "1 evaluation room" : `${totalPanels} rooms allocated`}
        />
        <Metric
          label="Total Pitches"
          value={panelsLoading ? <Skeleton className="h-7 w-12" /> : totalSlots}
          note="Teams to review"
        />
        <Metric
          label="Evaluations Done"
          value={panelsLoading ? <Skeleton className="h-7 w-12" /> : completedSlots}
          suffix={totalSlots > 0 ? `/ ${totalSlots}` : undefined}
          valueClassName="text-[var(--color-emerald)]"
        />
        <Metric
          label="Session Progress"
          value={panelsLoading ? <Skeleton className="h-7 w-14" /> : `${percentComplete}%`}
          valueClassName="text-[var(--color-blue)]"
        />
      </MetricGrid>

      {/* Active Evaluation Hero Banner */}
      {activeSlot ? (
        <section className="rounded-[16px] border border-[var(--color-blue-border)] bg-[var(--color-blue-dim)] p-5 sm:p-6 transition-all">
          <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-4">
            <div className="space-y-1">
              <div className="flex items-center gap-2">
                {activeSlot.slot.status === "in_progress" ? (
                  <Badge variant="coral" dot size="sm" className="font-semibold">
                    Presenting Now
                  </Badge>
                ) : (
                  <Badge variant="blue" size="sm" className="font-semibold">
                    Next Up ({activeSlot.slot.status})
                  </Badge>
                )}
                <span className="text-xs font-medium text-[var(--ink-secondary)]">
                  {activeSlot.panel.room} · {activeSlot.panel.panel_name}
                </span>
              </div>
              <div className="text-xl sm:text-2xl font-bold tracking-tight text-[var(--ink-primary)]">
                {activeSlot.slot.team_name}
              </div>
              <p className="text-xs text-[var(--ink-secondary)]">
                Slot #{activeSlot.slot.sequence_order} · Duration: ~{activeSlot.panel.slot_duration_mins} mins
              </p>
            </div>

            <div className="flex items-center gap-3">
              <Link
                href="/dashboard/judging"
                className="inline-flex h-9 items-center gap-1.5 rounded-[10px] bg-[var(--color-dark)] px-4 text-xs font-semibold text-[var(--ink-inverse)] transition-transform duration-150 active:scale-[0.97]"
              >
                <span>Grade Scorecard</span>
                <ArrowRight className="h-3.5 w-3.5" />
              </Link>
            </div>
          </div>
        </section>
      ) : null}

      {/* Panels Schedule Surface */}
      <section className="rounded-[16px] border border-[var(--card-line)] bg-[var(--card-solid)] p-4 sm:p-5 space-y-4">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-base font-semibold tracking-tight text-[var(--ink-primary)]">
              Assigned Panels & Staging
            </h2>
            <p className="text-xs text-[var(--ink-secondary)] mt-0.5">
              Overview of allocated rooms and pitches.
            </p>
          </div>
          <Link
            href="/dashboard/judging"
            className="text-xs font-medium text-[var(--color-blue)] hover:underline inline-flex items-center gap-1"
          >
            <span>Live Scoring Room</span>
            <ArrowRight className="h-3 w-3" />
          </Link>
        </div>

        {panelsLoading ? (
          <div className="space-y-3">
            <SkeletonCard />
            <SkeletonCard />
          </div>
        ) : panels.length === 0 ? (
          <EmptyState
            title="No evaluation panels assigned yet"
            description="Organizers have not assigned your judge account to an active judging room. Once a round starts, assigned panels will populate here."
          />
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {panels.map((panel) => {
              const panelDone = panel.total_slots > 0 && panel.completed_slots === panel.total_slots;
              const pct = panel.total_slots > 0 ? Math.round((panel.completed_slots / panel.total_slots) * 100) : 0;

              return (
                <div
                  key={panel.panel_id}
                  className="rounded-[14px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-4 space-y-3 flex flex-col justify-between"
                >
                  <div className="space-y-2">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <span className="font-semibold text-sm text-[var(--ink-primary)]">
                          {panel.room}
                        </span>
                        <Badge variant="subtle" size="sm">
                          {panel.panel_name}
                        </Badge>
                      </div>
                      <span className="text-xs font-mono font-medium text-[var(--ink-secondary)]">
                        {panel.completed_slots}/{panel.total_slots} done
                      </span>
                    </div>

                    <div className="text-xs text-[var(--ink-secondary)]">
                      Round: <strong className="text-[var(--ink-primary)]">{panel.round_name}</strong> · {panel.slot_duration_mins}m per pitch
                    </div>

                    {/* Progress bar */}
                    <div className="h-1.5 w-full rounded-full bg-[var(--card-line)] overflow-hidden">
                      <div
                        className="h-full rounded-full bg-[var(--color-blue)] transition-all duration-300"
                        style={{ width: `${pct}%` }}
                      />
                    </div>
                  </div>

                  {/* Team Slots mini preview */}
                  <div className="pt-2 border-t border-[var(--card-line)] space-y-1.5">
                    <span className="text-[11px] font-medium text-[var(--ink-tertiary)] block">
                      Scheduled Teams:
                    </span>
                    <div className="space-y-1 max-h-36 overflow-y-auto">
                      {panel.slots.map((s) => (
                        <div
                          key={s.slot_id}
                          className="flex items-center justify-between text-xs px-2.5 py-1.5 rounded-[8px] bg-[var(--card-solid)] border border-[var(--card-line)]"
                        >
                          <div className="flex items-center gap-2 truncate">
                            <span className="text-[10px] font-mono text-[var(--ink-tertiary)]">
                              #{s.sequence_order}
                            </span>
                            <span className="font-medium text-[var(--ink-primary)] truncate">
                              {s.team_name}
                            </span>
                          </div>
                          <div className="flex items-center gap-1.5 shrink-0">
                            {s.status === "in_progress" && (
                              <Badge variant="coral" size="sm" className="text-[10px] py-0 px-1.5 font-medium animate-pulse">
                                Live
                              </Badge>
                            )}
                            {s.is_my_scorecard_locked ? (
                              <span className="text-[10px] font-mono text-[var(--color-emerald)] flex items-center gap-0.5">
                                <CheckCircle2 className="h-3 w-3" /> Locked
                              </span>
                            ) : s.status === "completed" ? (
                              <span className="text-[10px] font-mono text-[var(--ink-tertiary)]">
                                Completed
                              </span>
                            ) : (
                              <span className="text-[10px] font-mono text-[var(--ink-secondary)]">
                                Scheduled
                              </span>
                            )}
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </section>
    </div>
  );
}
