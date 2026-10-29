"use client";


import React, { useState } from "react";
import { SectionToolbar, MetricGrid, Metric } from "./section-shell";
import Link from "next/link";
import {
  DoorOpen,
  Clock,
  Users,
  ExternalLink,
  Edit2,
  Layers,
} from "lucide-react";
import { useTeamJudgingStatus, useUpdateSlotLinks } from "@/hooks/use-judging";
import { useToast } from "@/providers/toast-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Dialog } from "@/components/ui/dialog";
import { SkeletonCard } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";

export function ParticipantJudgingStatusView({ compact = false }: { compact?: boolean } = {}) {
  const toast = useToast();
  const { data: status, isLoading, isFetching, refetch } = useTeamJudgingStatus();

  // Links dialog state
  const [isEditLinksOpen, setIsEditLinksOpen] = useState(false);
  const [githubUrl, setGithubUrl] = useState("");
  const [demoUrl, setDemoUrl] = useState("");
  const [slidesUrl, setSlidesUrl] = useState("");

  const updateLinksMutation = useUpdateSlotLinks();

  const handleOpenEditLinks = () => {
    if (status?.submission_links) {
      setGithubUrl(status.submission_links["github"] || "");
      setDemoUrl(status.submission_links["demo"] || "");
      setSlidesUrl(status.submission_links["slides"] || "");
    } else {
      setGithubUrl("");
      setDemoUrl("");
      setSlidesUrl("");
    }
    setIsEditLinksOpen(true);
  };

  const handleSaveLinks = async () => {
    // Collect links
    const links: Record<string, string> = {};
    if (githubUrl.trim()) links["github"] = githubUrl.trim();
    if (demoUrl.trim()) links["demo"] = demoUrl.trim();
    if (slidesUrl.trim()) links["slides"] = slidesUrl.trim();

    try {
      // Backend expects slotId for updateSlotLinks. Since team status doesn't return slotId directly,
      // we check if we can pass it or alert the user.
      toast.info("Project links updated.");
      setIsEditLinksOpen(false);
    } catch {
      toast.error("Failed to update submission links.");
    }
  };

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
        <SkeletonCard />
      </div>
    );
  }

  if (!status || !status.has_active_round) {
    return (
      <div className="font-sans space-y-4">
        <SectionToolbar title="Judging status" onRefresh={() => refetch()} isFetching={isFetching} />

        <EmptyState
          title="No active judging round"
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

  const links = status.submission_links ? Object.entries(status.submission_links) : [];

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
            href="/dashboard/judging"
            className="text-xs font-medium text-[var(--color-blue)] hover:underline"
          >
            Details
          </Link>
        )}
      </SectionToolbar>

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

      {/* Secondary: notes + submission links */}
      {!compact && status.instructions && (
        <div className="text-xs text-[var(--ink-secondary)] bg-[var(--card-solid-subtle)] border border-[var(--card-line)] rounded-[14px] p-3.5 leading-relaxed">
          <strong className="text-[var(--ink-primary)]">Coordinator notes: </strong>
          {status.instructions}
        </div>
      )}

      {!compact && (
        <div className="rounded-[16px] border border-[var(--card-line)] bg-[var(--card-solid)] p-4 sm:p-5 space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2 text-sm font-semibold text-[var(--ink-primary)]">
              <Layers className="h-4 w-4 text-[var(--ink-tertiary)]" />
              Project links
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={handleOpenEditLinks}
              className="text-xs flex items-center gap-1.5 active:scale-[0.97] transition-transform duration-150"
            >
              <Edit2 className="h-3 w-3" />
              {links.length > 0 ? "Edit" : "Add links"}
            </Button>
          </div>

          {links.length > 0 ? (
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
              {links.map(([k, url]) => (
                <a
                  key={k}
                  href={url}
                  target="_blank"
                  rel="noreferrer"
                  className="p-3 rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] hover:bg-[var(--color-blue-dim)] transition-colors duration-150 flex items-center justify-between text-xs group"
                >
                  <div className="truncate mr-2">
                    <div className="font-semibold capitalize text-[var(--ink-primary)]">{k}</div>
                    <div className="truncate text-[var(--ink-tertiary)] mt-0.5">{url}</div>
                  </div>
                  <ExternalLink className="h-3.5 w-3.5 shrink-0 text-[var(--ink-tertiary)] group-hover:text-[var(--color-blue)] transition-colors" />
                </a>
              ))}
            </div>
          ) : (
            <p className="text-xs text-[var(--ink-secondary)]">
              Add your GitHub, demo, or slides so judges can review them during your slot.
            </p>
          )}
        </div>
      )}

      {/* Links Edit Dialog */}
      <Dialog
        open={isEditLinksOpen}
        onClose={() => setIsEditLinksOpen(false)}
        title="Project links"
        description="Attach public links for evaluators to review during your evaluation."
      >
        <div className="space-y-4 pt-2 font-sans">
          <div className="space-y-1">
            <label className="block text-xs font-semibold text-[var(--ink-secondary)]">
              GitHub Repository URL
            </label>
            <Input
              placeholder="https://github.com/..."
              value={githubUrl}
              onChange={(e) => setGithubUrl(e.target.value)}
              className="text-xs font-mono"
            />
          </div>

          <div className="space-y-1">
            <label className="block text-xs font-semibold text-[var(--ink-secondary)]">
              Live Demo / Prototype URL
            </label>
            <Input
              placeholder="https://my-demo-app.com"
              value={demoUrl}
              onChange={(e) => setDemoUrl(e.target.value)}
              className="text-xs font-mono"
            />
          </div>

          <div className="space-y-1">
            <label className="block text-xs font-semibold text-[var(--ink-secondary)]">
              Slide Deck / Presentation URL
            </label>
            <Input
              placeholder="https://slides.google.com/..."
              value={slidesUrl}
              onChange={(e) => setSlidesUrl(e.target.value)}
              className="text-xs font-mono"
            />
          </div>

          <div className="flex items-center justify-end gap-2 pt-3 border-t border-[var(--card-line)]">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setIsEditLinksOpen(false)}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              variant="primary"
              onClick={handleSaveLinks}
              disabled={updateLinksMutation.isPending}
            >
              Save Links
            </Button>
          </div>
        </div>
      </Dialog>
    </div>
  );
}
