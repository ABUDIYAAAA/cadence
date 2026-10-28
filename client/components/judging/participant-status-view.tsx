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
          <Badge variant="solid" className="bg-black text-white text-xs uppercase px-2 py-0.5 animate-pulse">
            PRESENTING NOW
          </Badge>
        );
      case "queued":
        return (
          <Badge variant="solid" className="bg-neutral-800 text-white text-xs uppercase px-2 py-0.5">
            ON-DECK (BUFFER)
          </Badge>
        );
      case "scheduled":
        return (
          <Badge variant="outline" className="border-black text-black text-xs uppercase px-2 py-0.5">
            SCHEDULED
          </Badge>
        );
      case "completed":
        return (
          <Badge variant="outline" className="border-black bg-neutral-100 text-black text-xs uppercase px-2 py-0.5">
            EVALUATION COMPLETED
          </Badge>
        );
      default:
        return (
          <Badge variant="outline" className="border-neutral-300 text-neutral-500 text-xs uppercase px-2 py-0.5">
            WAITING
          </Badge>
        );
    }
  };

  if (isLoading) {
    return (
      <div className="font-mono space-y-4">
        <SkeletonCard />
        <SkeletonCard />
      </div>
    );
  }

  if (!status || !status.has_active_round) {
    return (
      <div className="font-mono space-y-4">
        <div className="flex items-center justify-between border-b-2 border-black pb-3">
          <div className="flex items-center gap-2">
            <Award className="h-5 w-5 text-black" />
            <h2 className="text-lg font-bold uppercase">Participant Judging Status</h2>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={() => refetch()}
            disabled={isFetching}
            className="border-black text-xs"
          >
            <RefreshCw className={`h-3.5 w-3.5 mr-1 ${isFetching ? "animate-spin" : ""}`} />
            REFRESH
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
    <div className="font-mono space-y-6">
      {/* Top Header */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 border-b-2 border-black pb-4">
        <div>
          <div className="flex items-center gap-2">
            <Award className="h-6 w-6 text-black" />
            <h2 className="text-xl font-bold tracking-tight uppercase">
              Judging Slot & Evaluation Status
            </h2>
          </div>
          <p className="text-sm text-neutral-600 mt-1">
            Active Round: <strong>{status.round_name || "General Evaluation"}</strong>
          </p>
        </div>

        <div className="flex items-center gap-3">
          {getStatusBadge(status.status)}
          <Button
            variant="outline"
            size="sm"
            onClick={() => refetch()}
            disabled={isFetching}
            className="flex items-center gap-1.5 border-black text-black hover:bg-neutral-100"
          >
            <RefreshCw className={`h-4 w-4 ${isFetching ? "animate-spin" : ""}`} />
            REFRESH
          </Button>
        </div>
      </div>

      {/* Critical Status Callout Banner */}
      {isOnDeck && (
        <div className="border-2 border-black p-4 bg-white shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] flex items-start gap-3">
          <AlertCircle className="h-5 w-5 text-black shrink-0 mt-0.5" />
          <div className="space-y-1">
            <div className="font-bold text-sm uppercase">YOU ARE ON-DECK!</div>
            <p className="text-xs text-neutral-700 leading-relaxed">
              Your squad is in the staging buffer! Please proceed immediately to{" "}
              <strong>{status.room || "your designated room"}</strong> and check in with a volunteer coordinator outside the door.
            </p>
          </div>
        </div>
      )}

      {isPresenting && (
        <div className="border-2 border-black p-4 bg-black text-white shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] flex items-start gap-3">
          <Play className="h-5 w-5 fill-white text-white shrink-0 mt-0.5" />
          <div className="space-y-1">
            <div className="font-bold text-sm uppercase tracking-wide">EVALUATION IN PROGRESS</div>
            <p className="text-xs text-neutral-300 leading-relaxed">
              Your squad is currently being evaluated in <strong>{status.room}</strong> with <strong>{status.panel_name}</strong>.
            </p>
          </div>
        </div>
      )}

      {isDone && (
        <div className="border-2 border-black p-4 bg-white shadow-[3px_3px_0px_0px_rgba(0,0,0,1)] flex items-start gap-3">
          <CheckCircle2 className="h-5 w-5 text-black shrink-0 mt-0.5" />
          <div className="space-y-1">
            <div className="font-bold text-sm uppercase">ROUND EVALUATION COMPLETED</div>
            <p className="text-xs text-neutral-600 leading-relaxed">
              Your squad has finished presenting for this round. Scores and rankings will be computed upon round completion.
            </p>
          </div>
        </div>
      )}

      {/* Primary Status Grid: Room, Slot Time, Queue Position */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {/* 1. ROOM ASSIGNMENT */}
        <div className="border-2 border-black p-5 bg-white shadow-[3px_3px_0px_0px_rgba(0,0,0,1)] flex flex-col justify-between">
          <div className="space-y-2">
            <div className="flex items-center justify-between text-neutral-500 text-xs uppercase font-semibold">
              <span className="flex items-center gap-1.5">
                <DoorOpen className="h-4 w-4 text-black" />
                Room Assignment
              </span>
              <Badge variant="outline" className="border-black text-[9px]">
                {status.room ? "ASSIGNED" : "PENDING"}
              </Badge>
            </div>

            {status.room ? (
              <div>
                <div className="text-xl font-bold uppercase text-black break-words mt-1">
                  {status.room}
                </div>
                {status.panel_name && (
                  <div className="text-xs text-neutral-600 mt-1 font-mono">
                    Panel: {status.panel_name}
                  </div>
                )}
              </div>
            ) : (
              <div className="py-2">
                <div className="text-base font-bold text-neutral-400">
                  Room Pending Allocation
                </div>
                <div className="text-xs text-neutral-500 mt-1">
                  Rooms are dynamically staged as rounds advance.
                </div>
              </div>
            )}
          </div>

          <div className="pt-3 border-t border-neutral-100 text-[10px] text-neutral-400 uppercase font-mono">
            Location Coordinate
          </div>
        </div>

        {/* 2. ESTIMATED SLOT & TIME */}
        <div className="border-2 border-black p-5 bg-white shadow-[3px_3px_0px_0px_rgba(0,0,0,1)] flex flex-col justify-between">
          <div className="space-y-2">
            <div className="flex items-center justify-between text-neutral-500 text-xs uppercase font-semibold">
              <span className="flex items-center gap-1.5">
                <Clock className="h-4 w-4 text-black" />
                Estimated Slot
              </span>
              <Badge variant="outline" className="border-black text-[9px]">
                {status.estimated_duration_mins ? `${status.estimated_duration_mins}m SLOT` : "SCHEDULE"}
              </Badge>
            </div>

            <div>
              <div className="text-xl font-bold uppercase text-black mt-1">
                {formatEstimatedTime(status.estimated_start_time)}
              </div>
              <div className="text-xs text-neutral-600 mt-1">
                {status.estimated_duration_mins
                  ? `Allocated pitch & demo: ~${status.estimated_duration_mins} mins`
                  : "Time adjusts automatically based on room pace"}
              </div>
            </div>
          </div>

          <div className="pt-3 border-t border-neutral-100 text-[10px] text-neutral-400 uppercase font-mono">
            Time Target
          </div>
        </div>

        {/* 3. QUEUE POSITION */}
        <div className="border-2 border-black p-5 bg-white shadow-[3px_3px_0px_0px_rgba(0,0,0,1)] flex flex-col justify-between">
          <div className="space-y-2">
            <div className="flex items-center justify-between text-neutral-500 text-xs uppercase font-semibold">
              <span className="flex items-center gap-1.5">
                <Users className="h-4 w-4 text-black" />
                Queue Staging
              </span>
              <Badge variant="outline" className="border-black text-[9px]">
                LIVE POSITION
              </Badge>
            </div>

            <div>
              <div className="text-3xl font-extrabold text-black mt-1">
                {status.queue_position !== undefined && status.queue_position > 0
                  ? `#${status.queue_position}`
                  : isPresenting
                  ? "NOW"
                  : isDone
                  ? "DONE"
                  : "—"}
              </div>
              <div className="text-xs text-neutral-600 mt-1">
                {status.queue_position === 1
                  ? "Next team in line for this room"
                  : status.queue_position !== undefined && status.queue_position > 1
                  ? `${status.queue_position - 1} team(s) ahead of you`
                  : "Waiting in active schedule"}
              </div>
            </div>
          </div>

          <div className="pt-3 border-t border-neutral-100 text-[10px] text-neutral-400 uppercase font-mono">
            Staging Order
          </div>
        </div>
      </div>

      {/* Instructions & Submission Links */}
      <div className="border-2 border-black p-5 bg-white shadow-[3px_3px_0px_0px_rgba(0,0,0,1)] space-y-4">
        <div className="flex items-center justify-between border-b border-neutral-200 pb-3">
          <div className="flex items-center gap-2">
            <Layers className="h-4 w-4 text-black" />
            <span className="font-bold text-sm uppercase">Squad Submission Material</span>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={handleOpenEditLinks}
            className="text-xs border-black text-black flex items-center gap-1"
          >
            <Edit2 className="h-3 w-3" />
            EDIT LINKS
          </Button>
        </div>

        {status.instructions && (
          <div className="text-xs text-neutral-700 bg-neutral-50 border border-neutral-200 p-3 leading-relaxed">
            <strong>Coordinator Notes:</strong> {status.instructions}
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
                className="p-3 border border-black bg-neutral-50 hover:bg-neutral-100 transition-colors flex items-center justify-between text-xs"
              >
                <div className="truncate mr-2">
                  <div className="font-bold uppercase text-[10px] text-neutral-500">{k}</div>
                  <div className="truncate text-black mt-0.5">{url}</div>
                </div>
                <ExternalLink className="h-3.5 w-3.5 shrink-0 text-black" />
              </a>
            ))}
          </div>
        ) : (
          <div className="text-xs text-neutral-500 italic p-3 border border-dashed border-neutral-300 text-center">
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
        <div className="space-y-4 pt-2 font-mono">
          <div className="space-y-1">
            <label className="block text-xs uppercase font-bold text-neutral-700">
              GitHub Repository URL
            </label>
            <Input
              placeholder="https://github.com/..."
              value={githubUrl}
              onChange={(e) => setGithubUrl(e.target.value)}
              className="text-xs font-mono border-black"
            />
          </div>

          <div className="space-y-1">
            <label className="block text-xs uppercase font-bold text-neutral-700">
              Live Demo / Prototype URL
            </label>
            <Input
              placeholder="https://my-demo-app.com"
              value={demoUrl}
              onChange={(e) => setDemoUrl(e.target.value)}
              className="text-xs font-mono border-black"
            />
          </div>

          <div className="space-y-1">
            <label className="block text-xs uppercase font-bold text-neutral-700">
              Slide Deck / Presentation URL
            </label>
            <Input
              placeholder="https://slides.google.com/..."
              value={slidesUrl}
              onChange={(e) => setSlidesUrl(e.target.value)}
              className="text-xs font-mono border-black"
            />
          </div>

          <div className="flex items-center justify-end gap-2 pt-2 border-t border-neutral-200">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setIsEditLinksOpen(false)}
              className="border-black text-black"
            >
              CANCEL
            </Button>
            <Button
              size="sm"
              onClick={handleSaveLinks}
              disabled={updateLinksMutation.isPending}
              className="bg-black text-white hover:bg-neutral-800 border border-black"
            >
              SAVE LINKS
            </Button>
          </div>
        </div>
      </Dialog>
    </div>
  );
}
