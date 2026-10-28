"use client";

import React from "react";
import {
  Award,
  Play,
  Clock,
  Users,
  Layers,
  Eye,
  Plus,
  DoorOpen,
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
  isActivatingId?: string | null;
}

export function RoundTable({
  rounds,
  isLoading,
  onActivate,
  onView,
  onAddPanel,
  onConfigureSlots,
  isActivatingId,
}: RoundTableProps) {
  if (isLoading) {
    return (
      <div className="border border-neutral-300 bg-white p-4 font-mono space-y-3">
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
    <div className="border border-neutral-300 bg-white overflow-hidden font-mono">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Round Name</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Queue Buffer</TableHead>
            <TableHead>Slot Duration</TableHead>
            <TableHead>Rooms / Panels</TableHead>
            <TableHead>Teams Scheduled</TableHead>
            <TableHead className="text-right">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rounds.map((round) => {
            const isActivating = isActivatingId === round.id;
            const isDraft = round.status === "draft";
            const isActive = round.status === "active";

            return (
              <TableRow key={round.id} className="group hover:bg-neutral-50/80">
                {/* Name */}
                <TableCell className="font-mono">
                  <div className="flex flex-col">
                    <div className="flex items-center gap-1.5 font-bold text-black text-sm">
                      <Award className="h-4 w-4 shrink-0 text-black" />
                      <span>{round.name}</span>
                    </div>
                    <span className="text-[10px] text-neutral-400 font-mono">
                      ID: {round.id.slice(0, 8)}...
                    </span>
                  </div>
                </TableCell>

                {/* Status */}
                <TableCell>
                  <Badge
                    variant={isActive ? "solid" : "subtle"}
                    size="sm"
                    className={
                      isActive
                        ? "bg-black text-white"
                        : "bg-neutral-100 text-neutral-700 border-neutral-300"
                    }
                  >
                    {round.status}
                  </Badge>
                </TableCell>

                {/* Buffer Size */}
                <TableCell className="text-xs">
                  <div className="flex items-center gap-1 font-semibold text-black">
                    <Users className="h-3.5 w-3.5 text-neutral-500" />
                    <span>{round.buffer_size || 2} in queue</span>
                  </div>
                  <span className="text-[10px] text-neutral-400">staging depth</span>
                </TableCell>

                {/* Slot Duration */}
                <TableCell className="text-xs">
                  <div className="flex items-center gap-1 font-semibold text-black">
                    <Clock className="h-3.5 w-3.5 text-neutral-500" />
                    <span>{round.slot_duration_mins || 10} mins</span>
                  </div>
                  <span className="text-[10px] text-neutral-400">per evaluation</span>
                </TableCell>

                {/* Panels count */}
                <TableCell className="text-xs">
                  <div className="flex items-center gap-1 font-semibold text-black">
                    <DoorOpen className="h-3.5 w-3.5 text-neutral-500" />
                    <span>{round.panel_count ?? 0} Rooms</span>
                  </div>
                </TableCell>

                {/* Slots count */}
                <TableCell className="text-xs">
                  <div className="flex items-center gap-1 font-semibold text-black">
                    <Layers className="h-3.5 w-3.5 text-neutral-500" />
                    <span>{round.slot_count ?? 0} Teams</span>
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
                      className="h-7 w-7 p-0"
                    >
                      <Eye className="h-3.5 w-3.5" />
                    </Button>

                    {/* Add Room/Panel */}
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => onAddPanel(round)}
                      title="Add Room / Judging Panel"
                      className="h-7 px-2 text-xs gap-1"
                    >
                      <Plus className="h-3 w-3" />
                      <span>Panel</span>
                    </Button>

                    {/* Configure Slots */}
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => onConfigureSlots(round)}
                      title="Assign Teams to Slots"
                      className="h-7 px-2 text-xs gap-1"
                    >
                      <Layers className="h-3 w-3" />
                      <span>Slots</span>
                    </Button>

                    {/* Activate Round (if draft) */}
                    {isDraft && (
                      <Button
                        variant="primary"
                        size="sm"
                        disabled={isActivating}
                        isLoading={isActivating}
                        onClick={() => onActivate(round.id)}
                        title="Activate Judging Round (Triggers Live Queue)"
                        className="h-7 px-2.5 text-xs gap-1"
                      >
                        <Play className="h-3 w-3" />
                        <span>Activate</span>
                      </Button>
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
