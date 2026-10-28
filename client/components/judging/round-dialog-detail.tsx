"use client";

import React from "react";
import { RoundSummary } from "@/types/judging";
import { useJudgingRound } from "@/hooks/use-judging";
import { Dialog } from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Award, Clock, Users, Play } from "lucide-react";

interface RoundDetailDialogProps {
  round: RoundSummary | null;
  open: boolean;
  onClose: () => void;
  onActivate: (roundId: string) => void;
  isActivating?: boolean;
}

export function RoundDetailDialog({
  round,
  open,
  onClose,
  onActivate,
  isActivating,
}: RoundDetailDialogProps) {
  const { data: roundDetail, isLoading } = useJudgingRound(round?.id || "");

  if (!round || !open) return null;

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Judging Round Rubrics & Parameters"
      description={`Round UUID: ${round.id}`}
      footer={
        <div className="flex items-center justify-between w-full">
          <div>
            {round.status === "draft" && (
              <Button
                variant="primary"
                size="sm"
                disabled={isActivating}
                isLoading={isActivating}
                onClick={() => {
                  onClose();
                  onActivate(round.id);
                }}
                className="gap-1.5 text-xs"
              >
                <Play className="h-3.5 w-3.5" />
                <span>Activate Round</span>
              </Button>
            )}
          </div>

          <Button variant="outline" size="sm" onClick={onClose} className="text-xs">
            Close
          </Button>
        </div>
      }
    >
      <div className="space-y-4 font-mono text-xs max-h-[70vh] overflow-y-auto pr-1">
        {/* Header Metadata */}
        <div className="border border-neutral-300 divide-y divide-neutral-200 bg-neutral-50">
          <div className="flex justify-between p-2.5">
            <span className="text-neutral-500">Round Title</span>
            <strong className="text-black">{round.name}</strong>
          </div>

          <div className="flex justify-between p-2.5">
            <span className="text-neutral-500">Status</span>
            <Badge
              variant={round.status === "active" ? "solid" : "subtle"}
              size="sm"
            >
              {round.status}
            </Badge>
          </div>

          <div className="flex justify-between p-2.5">
            <span className="text-neutral-500 flex items-center gap-1">
              <Users className="h-3.5 w-3.5 text-neutral-400" />
              <span>Queue Staging Buffer</span>
            </span>
            <span className="font-bold text-black">{round.buffer_size} Teams</span>
          </div>

          <div className="flex justify-between p-2.5">
            <span className="text-neutral-500 flex items-center gap-1">
              <Clock className="h-3.5 w-3.5 text-neutral-400" />
              <span>Slot Duration</span>
            </span>
            <span className="font-bold text-black">{round.slot_duration_mins} Minutes per team</span>
          </div>

          <div className="flex justify-between p-2.5">
            <span className="text-neutral-500">Established</span>
            <span>{new Date(round.created_at).toLocaleString()}</span>
          </div>
        </div>

        {/* Criteria Rubrics */}
        <div className="border border-neutral-300 bg-white overflow-hidden space-y-1">
          <div className="p-2.5 bg-neutral-100 border-b border-neutral-300 font-bold uppercase text-[11px] flex items-center justify-between">
            <span className="flex items-center gap-1.5">
              <Award className="h-3.5 w-3.5" />
              <span>Scoring Criteria & Weightings</span>
            </span>
          </div>

          {isLoading || !roundDetail ? (
            <div className="p-4 space-y-2">
              <Skeleton className="h-4 w-1/3" />
              <Skeleton className="h-16 w-full" />
            </div>
          ) : roundDetail.criteria.length === 0 ? (
            <div className="p-4 text-center text-neutral-400 italic">
              No evaluation criteria loaded for this round.
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>#</TableHead>
                  <TableHead>Criterion</TableHead>
                  <TableHead>Max Score</TableHead>
                  <TableHead>Weight</TableHead>
                  <TableHead>Description</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {roundDetail.criteria.map((c, idx) => (
                  <TableRow key={c.id || idx}>
                    <TableCell className="font-bold">{c.order_index || idx + 1}</TableCell>
                    <TableCell className="font-bold text-black">{c.name}</TableCell>
                    <TableCell>{c.max_score} pts</TableCell>
                    <TableCell>×{c.weight}</TableCell>
                    <TableCell className="text-neutral-600 text-[11px]">
                      {c.description || "—"}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </div>
      </div>
    </Dialog>
  );
}
