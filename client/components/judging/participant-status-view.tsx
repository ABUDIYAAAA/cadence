"use client";


import React from "react";
import { SectionToolbar, MetricGrid, Metric } from "./section-shell";
import Link from "next/link";
import {
  DoorOpen,
  Clock,
  Users,
} from "lucide-react";
import { useTeamJudgingStatus } from "@/hooks/use-judging";
import { useTenant } from "@/providers/tenant-provider";
import { getCompetitionSectionHref } from "@/lib/competition-utils";
import { Select } from "@/components/ui/select";
import { SkeletonCard } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";

export function ParticipantJudgingStatusView({
  compact = false,
  hideCompetitionSwitcher = false,
}: {
  compact?: boolean;
  hideCompetitionSwitcher?: boolean;
} = {}) {
  const { competitionSlug, activeCompetition, competitions, selectCompetition } = useTenant();
  const { data: status, isLoading, isFetching, refetch } = useTeamJudgingStatus();

  const competitionSelector =
    !hideCompetitionSwitcher && competitions.length > 0 ? (
      <div className="space-y-1.5">
        <div className="flex items-center justify-between gap-2">
          <span className="text-xs font-semibold text-[var(--ink-primary)]">
            Competition
          </span>
        </div>
        <Select
          aria-label="Select competition for judging status"
          value={competitionSlug ?? ""}
          onChange={(e) => {
            if (e.target.value && e.target.value !== competitionSlug) {
              selectCompetition(e.target.value);
            }
          }}
          className="h-10 w-full text-xs font-medium"
          options={competitions.map((comp) => ({
            value: comp.slug,
            label: comp.name,
          }))}
        />
      </div>
    ) : null;

  const formatEstimatedTime = (iso?: string | null) => {
    if (!iso) return "TBD";
    try {
      const date = new Date(iso);
      return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
    } catch {
      return "TBD";
    }
  };

  if (isLoading) {
    return (
      <div className="font-sans space-y-4">
        {competitionSelector}
        <SkeletonCard />
      </div>
    );
  }

  const compName = activeCompetition?.name || competitionSlug;

  if (!status || !status.has_active_round) {
    return (
      <div className="font-sans space-y-4">
        <SectionToolbar title="Judging status" onRefresh={() => refetch()} isFetching={isFetching} />
        {competitionSelector}

        <EmptyState
          title={compName ? `No active judging round for ${compName}` : "No active judging round"}
          description="Once organizers start a round, your room, panel, and time slot will show up here."
        />
      </div>
    );
  }

  const isOnDeck = status.status === "queued";
  const isPresenting = status.status === "in_progress";
  const isDone = status.status === "completed";

  const hasQueue = status.queue_position !== undefined && status.queue_position > 0;
  const timeLabel = formatEstimatedTime(status.estimated_start_time);

  // The single most important message: what should I do right now?
  const hero = isPresenting
    ? {
        tone: "blue",
        label: "Presenting now",
        title: "Your team is being evaluated",
        body: `${status.room || "Your room"}${status.panel_name ? ` · ${status.panel_name}` : ""}`,
      }
    : isOnDeck
    ? {
        tone: "coral",
        label: "On deck",
        title: `Head to ${status.room || "your room"} now`,
        body: "Check in with the volunteer outside the door. You present next.",
      }
    : isDone
    ? {
        tone: "emerald",
        label: "Complete",
        title: "You've finished this round",
        body: "Scores and rankings appear once the round closes.",
      }
    : {
        tone: "subtle",
        label: "Scheduled",
        title: status.estimated_start_time ? `You present around ${timeLabel}` : "Waiting for your slot",
        body: status.room
          ? `Be at ${status.room} 10 minutes early.`
          : "Your room will be assigned as the round advances.",
      };

  const heroStyles: Record<string, string> = {
    blue: "border-[var(--color-blue-border)] bg-[var(--color-blue-dim)]",
    coral: "border-[#CE6969]/30 bg-[var(--color-coral-dim)]",
    emerald: "border-[var(--color-emerald-border)] bg-[var(--color-emerald-dim)]",
    subtle: "border-[var(--card-line)] bg-[var(--card-solid)]",
  };

  const ahead = (status.queue_position ?? 1) - 1;
  const facts = [
    {
      icon: DoorOpen,
      label: "Room",
      value: status.room || "Pending",
      note: status.panel_name || "Assigned as rounds advance",
    },
    {
      icon: Clock,
      label: "Start time",
      value: timeLabel,
      note: status.estimated_duration_mins
        ? `${status.estimated_duration_mins} min slot`
        : "Adjusts to room pace",
    },
    {
      icon: Users,
      label: "Queue",
      value: hasQueue ? `#${status.queue_position}` : isPresenting ? "Now" : isDone ? "Done" : "—",
      note:
        status.queue_position === 1
          ? "You're next"
          : hasQueue
          ? `${ahead} team${ahead === 1 ? "" : "s"} ahead`
          : "Not queued yet",
    },
  ];


  return (
    <div className="font-sans space-y-6">
      <SectionToolbar
        title="Judging status"
        subtitle={status.round_name || "General evaluation"}
        onRefresh={() => refetch()}
        isFetching={isFetching}
      >
        {compact && (
          <Link
            href={
              competitionSlug
                ? getCompetitionSectionHref(competitionSlug, "judging")
                : "/dashboard/judging"
            }
            className="text-xs font-medium text-[var(--color-blue)] hover:underline"
          >
            Details
          </Link>
        )}
      </SectionToolbar>

      {competitionSelector}

      <MetricGrid cols={3}>
        {facts.map((f) => (
          <Metric key={f.label} label={f.label} value={f.value} note={f.note} />
        ))}
      </MetricGrid>

      {/* Primary surface: what to do right now */}
      <div className={`rounded-[16px] border p-4 sm:p-5 ${heroStyles[hero.tone]}`}>
        <div className="flex items-center gap-2 mb-1.5">
          {isPresenting && (
            <span className="relative flex h-2 w-2">
              <span className="absolute inline-flex h-full w-full rounded-full bg-[var(--color-blue)] opacity-60 animate-ping" />
              <span className="relative inline-flex h-2 w-2 rounded-full bg-[var(--color-blue)]" />
            </span>
          )}
          <span className="text-xs font-medium text-[var(--ink-secondary)]">{hero.label}</span>
        </div>
        <div className="text-base font-semibold text-[var(--ink-primary)] text-balance">{hero.title}</div>
        <p className="text-xs text-[var(--ink-secondary)] mt-1">{hero.body}</p>
      </div>

      {/* Secondary: notes */}
      {!compact && status.instructions && (
        <div className="text-xs text-[var(--ink-secondary)] bg-[var(--card-solid-subtle)] border border-[var(--card-line)] rounded-[14px] p-3.5 leading-relaxed">
          <strong className="text-[var(--ink-primary)]">Coordinator notes: </strong>
          {status.instructions}
        </div>
      )}

    </div>
  );
}
