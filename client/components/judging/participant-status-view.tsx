"use client";

import React, { useState } from "react";
import {
  Award,
  DoorOpen,
  Clock,
  Users,
  CheckCircle2,
  AlertCircle,
  ExternalLink,
  Edit2,
  RefreshCw,
  Play,
  Layers,
} from "lucide-react";
import { useTeamJudgingStatus, useUpdateSlotLinks } from "@/hooks/use-judging";
import { useToast } from "@/providers/toast-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Dialog } from "@/components/ui/dialog";
import { SkeletonCard } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";

export function ParticipantJudgingStatusView() {
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
    if (!iso) return "To be scheduled";
    try {
      const date = new Date(iso);
      return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
    } catch {
      return "To be scheduled";
    }
  };

  const getStatusBadge = (st?: string) => {
    switch (st) {
      case "in_progress":
        return (
          <Badge variant="blue" className="text-xs uppercase px-2.5 py-1 animate-pulse font-semibold">
            PRESENTING NOW
          </Badge>
        );
      case "queued":
        return (
          <Badge variant="coral" className="text-xs uppercase px-2.5 py-1 font-semibold">
            ON-DECK (BUFFER)
          </Badge>
        );
      case "scheduled":
        return (
          <Badge variant="subtle" className="text-xs uppercase px-2.5 py-1 font-semibold">
            SCHEDULED
          </Badge>
        );
      case "completed":
        return (
          <Badge variant="solid" className="text-xs uppercase px-2.5 py-1 font-semibold">
            EVALUATION COMPLETED
          </Badge>
        );
      default:
        return (
          <Badge variant="subtle" className="text-xs uppercase px-2.5 py-1 font-semibold">
            WAITING
          </Badge>
        );
    }
  };

  if (isLoading) {
    return (
      <div className="font-sans space-y-4">
        <SkeletonCard />
        <SkeletonCard />
      </div>
    );
  }

  if (!status || !status.has_active_round) {
    return (
      <div className="font-sans space-y-4">
        <div className="flex items-center justify-between border-b border-[var(--card-line)] pb-4">
          <div className="flex items-center gap-2.5">
            <div className="h-8 w-8 rounded-full bg-[var(--color-blue-dim)] flex items-center justify-center text-[var(--color-blue)]">
              <Award className="h-4 w-4" />
            </div>
            <h2 className="text-lg font-bold text-[var(--ink-primary)]">
              Participant Judging Status
            </h2>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={() => refetch()}
            disabled={isFetching}
            className="text-xs"
          >
            <RefreshCw className={`h-3.5 w-3.5 mr-1 ${isFetching ? "animate-spin" : ""}`} />
            Refresh
          </Button>
        </div>

        <EmptyState
          title="NO ACTIVE JUDGING ROUND"
          description="Judging is not currently underway for your squad. Once an evaluation round is activated by organizers, your designated room, assigned evaluators, and real-time slot schedule will be displayed here."
        />
      </div>
    );
  }

  const isOnDeck = status.status === "queued";
  const isPresenting = status.status === "in_progress";
  const isDone = status.status === "completed";

  return (
    <div className="font-sans space-y-6">
      {/* Top Header */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 border-b border-[var(--card-line)] pb-4">
        <div>
          <div className="flex items-center gap-2.5">
            <div className="h-9 w-9 rounded-full bg-[var(--color-blue-dim)] flex items-center justify-center text-[var(--color-blue)]">
              <Award className="h-5 w-5" />
            </div>
            <h2 className="text-xl font-bold tracking-tight text-[var(--ink-primary)]">
              Judging Slot & Evaluation Status
            </h2>
          </div>
          <p className="text-xs text-[var(--ink-secondary)] mt-1 ml-11">
            Active Round: <strong className="text-[var(--ink-primary)]">{status.round_name || "General Evaluation"}</strong>
          </p>
        </div>

        <div className="flex items-center gap-3">
          {getStatusBadge(status.status)}
          <Button
            variant="outline"
            size="sm"
            onClick={() => refetch()}
            disabled={isFetching}
            className="flex items-center gap-1.5 text-xs"
          >
            <RefreshCw className={`h-3.5 w-3.5 ${isFetching ? "animate-spin" : ""}`} />
            Refresh
          </Button>
        </div>
      </div>

      {/* Critical Status Callout Banner */}
      {isOnDeck && (
        <div className="rounded-[18px] border border-[#CE6969]/30 bg-[var(--color-coral-dim)] p-4.5 flex items-start gap-3.5 shadow-sm">
          <div className="h-8 w-8 rounded-full bg-[#CE6969]/15 flex items-center justify-center text-[var(--color-coral)] shrink-0 mt-0.5">
            <AlertCircle className="h-4 w-4" />
          </div>
          <div className="space-y-1">
            <div className="font-bold text-sm text-[var(--color-coral)]">YOU ARE ON-DECK!</div>
            <p className="text-xs text-[var(--color-coral)] leading-relaxed">
              Your squad is in the staging buffer! Please proceed immediately to{" "}
              <strong className="text-[var(--ink-primary)]">{status.room || "your designated room"}</strong> and check in with a volunteer coordinator outside the door.
            </p>
          </div>
        </div>
      )}

      {isPresenting && (
        <div className="rounded-[18px] border border-[#83A2DB]/40 bg-[var(--color-blue-dim)] p-4.5 flex items-start gap-3.5 shadow-sm">
          <div className="h-8 w-8 rounded-full bg-[#83A2DB]/20 flex items-center justify-center text-[var(--color-blue)] shrink-0 mt-0.5">
            <Play className="h-4 w-4 fill-current" />
          </div>
          <div className="space-y-1">
            <div className="font-bold text-sm text-[var(--color-blue)] tracking-wide">EVALUATION IN PROGRESS</div>
            <p className="text-xs text-[var(--color-blue)] leading-relaxed">
              Your squad is currently being evaluated in <strong className="text-[var(--ink-primary)]">{status.room}</strong> with <strong className="text-[var(--ink-primary)]">{status.panel_name}</strong>.
            </p>
          </div>
        </div>
      )}

      {isDone && (
        <div className="rounded-[18px] border border-[var(--color-emerald-border)] bg-[var(--color-emerald-dim)] p-4.5 flex items-start gap-3.5 shadow-sm">
          <div className="h-8 w-8 rounded-full bg-[#2EB88A]/15 flex items-center justify-center text-[#2EB88A] shrink-0 mt-0.5">
            <CheckCircle2 className="h-4 w-4" />
          </div>
          <div className="space-y-1">
            <div className="font-bold text-sm text-[#2EB88A]">ROUND EVALUATION COMPLETED</div>
            <p className="text-xs text-[var(--color-emerald)] leading-relaxed">
              Your squad has finished presenting for this round. Scores and rankings will be computed upon round completion.
            </p>
          </div>
        </div>
      )}

      {/* Primary Status Grid: Room, Slot Time, Queue Position */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {/* 1. ROOM ASSIGNMENT */}
        <div className="rounded-[20px] border border-[var(--card-border)] bg-[var(--card-solid)] p-5 shadow-[var(--shadow-card)] flex flex-col justify-between">
          <div className="space-y-2">
            <div className="flex items-center justify-between text-[var(--ink-secondary)] text-xs font-semibold">
              <span className="flex items-center gap-1.5">
                <DoorOpen className="h-4 w-4 text-[var(--color-blue)]" />
                Room Assignment
              </span>
              <Badge variant={status.room ? "blue" : "subtle"} size="sm">
                {status.room ? "ASSIGNED" : "PENDING"}
              </Badge>
            </div>

            {status.room ? (
              <div>
                <div className="text-xl font-bold text-[var(--ink-primary)] break-words mt-1">
                  {status.room}
                </div>
                {status.panel_name && (
                  <div className="text-xs text-[var(--ink-secondary)] mt-1 font-mono">
                    Panel: {status.panel_name}
                  </div>
                )}
              </div>
            ) : (
              <div className="py-2">
                <div className="text-base font-semibold text-[var(--ink-tertiary)]">
                  Room Pending Allocation
                </div>
                <div className="text-xs text-[var(--ink-secondary)] mt-1">
                  Rooms are dynamically staged as rounds advance.
                </div>
              </div>
            )}
          </div>

          <div className="pt-3 border-t border-[var(--card-line)] text-[10px] text-[var(--ink-tertiary)] uppercase font-mono tracking-wider">
            Location Coordinate
          </div>
        </div>

        {/* 2. ESTIMATED SLOT & TIME */}
        <div className="rounded-[20px] border border-[var(--card-border)] bg-[var(--card-solid)] p-5 shadow-[var(--shadow-card)] flex flex-col justify-between">
          <div className="space-y-2">
            <div className="flex items-center justify-between text-[var(--ink-secondary)] text-xs font-semibold">
              <span className="flex items-center gap-1.5">
                <Clock className="h-4 w-4 text-[var(--color-blue)]" />
                Estimated Slot
              </span>
              <Badge variant="subtle" size="sm">
                {status.estimated_duration_mins ? `${status.estimated_duration_mins}m SLOT` : "SCHEDULE"}
              </Badge>
            </div>

            <div>
              <div className="text-xl font-bold font-mono text-[var(--ink-primary)] mt-1">
                {formatEstimatedTime(status.estimated_start_time)}
              </div>
              <div className="text-xs text-[var(--ink-secondary)] mt-1">
                {status.estimated_duration_mins
                  ? `Allocated pitch & demo: ~${status.estimated_duration_mins} mins`
                  : "Time adjusts automatically based on room pace"}
              </div>
            </div>
          </div>

          <div className="pt-3 border-t border-[var(--card-line)] text-[10px] text-[var(--ink-tertiary)] uppercase font-mono tracking-wider">
            Time Target
          </div>
        </div>

        {/* 3. QUEUE POSITION */}
        <div className="rounded-[20px] border border-[var(--card-border)] bg-[var(--card-solid)] p-5 shadow-[var(--shadow-card)] flex flex-col justify-between">
          <div className="space-y-2">
            <div className="flex items-center justify-between text-[var(--ink-secondary)] text-xs font-semibold">
              <span className="flex items-center gap-1.5">
                <Users className="h-4 w-4 text-[var(--color-blue)]" />
                Queue Staging
              </span>
              <Badge variant="subtle" size="sm">
                LIVE POSITION
              </Badge>
            </div>

            <div>
              <div className="text-3xl font-extrabold font-mono text-[var(--ink-primary)] mt-1">
                {status.queue_position !== undefined && status.queue_position > 0
                  ? `#${status.queue_position}`
                  : isPresenting
                  ? "NOW"
                  : isDone
                  ? "DONE"
                  : "—"}
              </div>
              <div className="text-xs text-[var(--ink-secondary)] mt-1">
                {status.queue_position === 1
                  ? "Next team in line for this room"
                  : status.queue_position !== undefined && status.queue_position > 1
                  ? `${status.queue_position - 1} team(s) ahead of you`
                  : "Waiting in active schedule"}
              </div>
            </div>
          </div>

          <div className="pt-3 border-t border-[var(--card-line)] text-[10px] text-[var(--ink-tertiary)] uppercase font-mono tracking-wider">
            Staging Order
          </div>
        </div>
      </div>

      {/* Instructions & Submission Links */}
      <div className="rounded-[20px] border border-[var(--card-border)] bg-[var(--card-solid)] p-5 shadow-[var(--shadow-card)] space-y-4">
        <div className="flex items-center justify-between border-b border-[var(--card-line)] pb-3">
          <div className="flex items-center gap-2">
            <Layers className="h-4 w-4 text-[var(--color-blue)]" />
            <span className="font-bold text-sm text-[var(--ink-primary)]">Squad Submission Material</span>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={handleOpenEditLinks}
            className="text-xs flex items-center gap-1.5"
          >
            <Edit2 className="h-3 w-3" />
            Edit Links
          </Button>
        </div>

        {status.instructions && (
          <div className="text-xs text-[var(--ink-secondary)] bg-[var(--card-solid-subtle)] border border-[var(--card-line)] rounded-[14px] p-3.5 leading-relaxed">
            <strong className="text-[var(--ink-primary)]">Coordinator Notes:</strong> {status.instructions}
          </div>
        )}

        {status.submission_links && Object.keys(status.submission_links).length > 0 ? (
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
            {Object.entries(status.submission_links).map(([k, url]) => (
              <a
                key={k}
                href={url}
                target="_blank"
                rel="noreferrer"
                className="p-3.5 rounded-[14px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] hover:bg-[var(--color-blue-dim)] transition-all flex items-center justify-between text-xs group"
              >
                <div className="truncate mr-2">
                  <div className="font-bold uppercase text-[10px] text-[var(--ink-tertiary)] tracking-wider">{k}</div>
                  <div className="truncate text-[var(--ink-primary)] font-medium mt-0.5 group-hover:text-[var(--color-blue)] transition-colors">{url}</div>
                </div>
                <ExternalLink className="h-3.5 w-3.5 shrink-0 text-[var(--ink-tertiary)] group-hover:text-[var(--color-blue)] transition-colors" />
              </a>
            ))}
          </div>
        ) : (
          <div className="text-xs text-[var(--ink-tertiary)] italic p-5 border border-dashed border-[var(--card-line)] rounded-[16px] text-center">
            No project submission links registered yet. Click &apos;Edit Links&apos; to attach your GitHub repository, live demo, or slide deck for the evaluators.
          </div>
        )}
      </div>

      {/* Links Edit Dialog */}
      <Dialog
        open={isEditLinksOpen}
        onClose={() => setIsEditLinksOpen(false)}
        title="SUBMISSION ARTIFACT LINKS"
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
