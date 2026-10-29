"use client";

import React from "react";
import {
  Play,
  Clock,
  Users,
  Layers,
  Eye,
  Plus,
  DoorOpen,
  ArrowRightLeft,
  Square,
  Trash2,
} from "lucide-react";
import { RoundSummary } from "@/types/judging";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";

interface RoundTableProps {
  rounds: RoundSummary[];
  isLoading: boolean;
  onActivate: (roundId: string) => void;
  onView: (round: RoundSummary) => void;
  onAddPanel: (round: RoundSummary) => void;
  onConfigureSlots: (round: RoundSummary) => void;
  onReassignTeams: (round: RoundSummary) => void;
  onEnd: (round: RoundSummary) => void;
  onDelete: (round: RoundSummary) => void;
  isActivatingId?: string | null;
  isAdmin?: boolean;
}

export function RoundTable({
  rounds,
  isLoading,
  onActivate,
  onView,
  onAddPanel,
  onConfigureSlots,
  onReassignTeams,
  onEnd,
  onDelete,
  isActivatingId,
  isAdmin = true,
}: RoundTableProps) {
  if (isLoading) {
    return (
      <div className="space-y-3 font-sans">
        <div className="space-y-2">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="flex items-center space-x-4 py-2">
              <Skeleton className="h-4 w-1/4" />
              <Skeleton className="h-4 w-1/6" />
              <Skeleton className="h-4 w-1/6" />
              <Skeleton className="h-4 w-1/6" />
              <Skeleton className="h-4 w-1/6" />
            </div>
          ))}
        </div>
      </div>
    );
  }

  if (rounds.length === 0) {
    return (
      <EmptyState
        title="No Judging Rounds Found"
        description="No judging rounds have been established yet. Click 'Create Judging Round' to configure scoring criteria, rooms, and team allocations."
      />
    );
  }

  return (
    <div className="w-full overflow-x-auto font-sans">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Round Name</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>On deck</TableHead>
            <TableHead>Time per team</TableHead>
            <TableHead>Rooms</TableHead>
            <TableHead>Teams</TableHead>
            <TableHead className="text-right">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rounds.map((round) => {
            const isActivating = isActivatingId === round.id;
            const isDraft = round.status === "draft";
            const isActive = round.status === "active";
            const isFinished = round.status === "completed";

            return (
              <TableRow key={round.id} className="group">
                {/* Name */}
                <TableCell>
                  <div className="flex flex-col">
                    <span className="font-semibold text-[var(--ink-primary)] text-sm">
                      {round.name}
                    </span>
                  </div>
                </TableCell>

                {/* Status */}
                <TableCell>
                  <Badge
                    variant={isActive ? "blue" : "subtle"}
                    size="sm"
                  >
                    {round.status}
                  </Badge>
                </TableCell>

                {/* Buffer Size */}
                <TableCell className="text-xs">
                  <div className="flex items-center gap-1 font-semibold text-[var(--ink-primary)]">
                    <Users className="h-3.5 w-3.5 text-[var(--ink-secondary)]" />
                    <span>{round.buffer_size || 2} teams</span>
                  </div>
                  <span className="text-[10px] text-[var(--ink-tertiary)]">per room</span>
                </TableCell>

                {/* Slot Duration */}
                <TableCell className="text-xs">
                  <div className="flex items-center gap-1 font-semibold text-[var(--ink-primary)]">
                    <Clock className="h-3.5 w-3.5 text-[var(--ink-secondary)]" />
                    <span>{round.slot_duration_mins || 10} min</span>
                  </div>
                  <span className="text-[10px] text-[var(--ink-tertiary)]">pitch + Q&amp;A</span>
                </TableCell>

                {/* Panels count */}
                <TableCell className="text-xs">
                  <div className="flex items-center gap-1 font-semibold text-[var(--ink-primary)]">
                    <DoorOpen className="h-3.5 w-3.5 text-[var(--ink-secondary)]" />
                    <span>{round.panel_count ?? 0} {round.panel_count === 1 ? "room" : "rooms"}</span>
                  </div>
                </TableCell>

                {/* Slots count */}
                <TableCell className="text-xs">
                  <div className="flex items-center gap-1 font-semibold text-[var(--ink-primary)]">
                    <Layers className="h-3.5 w-3.5 text-[var(--ink-secondary)]" />
                    <span>{round.slot_count ?? 0} {round.slot_count === 1 ? "team" : "teams"}</span>
                  </div>
                </TableCell>

                {/* Actions */}
                <TableCell className="text-right">
                  <div className="flex items-center justify-end gap-1.5">
                    {/* View Details */}
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => onView(round)}
                      title="Inspect Round Parameters & Criteria"
                      className="h-7 w-7 p-0 text-[var(--ink-secondary)] hover:text-[var(--ink-primary)]"
                    >
                      <Eye className="h-3.5 w-3.5" />
                    </Button>

                    {isAdmin && (
                      <>
                        {!isFinished && (
                          <>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => onAddPanel(round)}
                              title="Add a room and choose its judges"
                              className="h-7 px-2 text-xs gap-1 text-[var(--ink-secondary)] hover:text-[var(--ink-primary)]"
                            >
                              <Plus className="h-3 w-3" />
                              <span>Add room</span>
                            </Button>

                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => onConfigureSlots(round)}
                              title="Add teams to a room"
                              className="h-7 px-2 text-xs gap-1 text-[var(--ink-secondary)] hover:text-[var(--ink-primary)]"
                            >
                              <Layers className="h-3 w-3" />
                              <span>Add teams</span>
                            </Button>
                          </>
                        )}

                        {/* Teams waiting in the queue can be moved until the round ends */}
                        {!isFinished && (
                        <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => onReassignTeams(round)}
                            title="Move teams between rooms or change their order"
                            className="h-7 px-2 text-xs gap-1 text-[var(--ink-secondary)] hover:text-[var(--ink-primary)]"
                          >
                            <ArrowRightLeft className="h-3 w-3" />
                            <span>Move teams</span>
                          </Button>
                        )}

                        {isDraft && (
                          <Button
                            variant="primary"
                            size="sm"
                            disabled={isActivating}
                            isLoading={isActivating}
                            onClick={() => onActivate(round.id)}
                            title="Start the round: judges can begin scoring"
                            className="h-7 px-2.5 text-xs gap-1"
                          >
                            <Play className="h-3 w-3" />
                            <span>Start round</span>
                          </Button>
                        )}

                        {isActive && (
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => onEnd(round)}
                            title="End the round: judges can no longer score"
                            className="h-7 px-2.5 text-xs gap-1"
                          >
                            <Square className="h-3 w-3" />
                            <span>End round</span>
                          </Button>
                        )}

                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => onDelete(round)}
                          title="Delete this round"
                          aria-label={`Delete ${round.name}`}
                          className="h-7 w-7 p-0 text-[var(--color-coral)] hover:bg-[#CE6969]/10"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      </>
                    )}
                  </div>
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </div>
  );
}
