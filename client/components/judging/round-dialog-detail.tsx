"use client";

import React from "react";
import { RoundSummary } from "@/types/judging";
import { useJudgingRound, useRoundPanels } from "@/hooks/use-judging";
import { Dialog } from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Play } from "lucide-react";

interface RoundDetailDialogProps {
  round: RoundSummary | null;
  open: boolean;
  onClose: () => void;
  onActivate: (roundId: string) => void;
  isActivating?: boolean;
}

const STATUS_LABEL: Record<string, string> = {
  draft: "Not started",
  active: "In progress",
  completed: "Ended",
};

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="space-y-2">
      <h3 className="text-[11px] font-semibold uppercase tracking-wider text-[var(--ink-secondary)]">{title}</h3>
      {children}
    </section>
  );
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-3 px-3.5 py-2.5">
      <span className="text-[var(--ink-secondary)]">{label}</span>
      <span className="font-semibold text-[var(--ink-primary)] text-right">{children}</span>
    </div>
  );
}

export function RoundDetailDialog({
  round,
  open,
  onClose,
  onActivate,
  isActivating,
}: RoundDetailDialogProps) {
  const { data: roundDetail, isLoading } = useJudgingRound(open ? round?.id || "" : "");
  const { data: rooms, isLoading: roomsLoading } = useRoundPanels(open ? round?.id : undefined);

  if (!round || !open) return null;

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Round details"
      description={round.name}
      className="max-w-2xl"
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
                <span>Start round</span>
              </Button>
            )}
          </div>
          <Button variant="outline" size="sm" onClick={onClose} className="text-xs">
            Close
          </Button>
        </div>
      }
    >
      <div className="space-y-5 font-sans text-xs max-h-[70vh] overflow-y-auto pr-1">
        <Section title="Settings">
          <div className="rounded-[14px] border border-[var(--card-line)] divide-y divide-[var(--card-line)] bg-[var(--card-solid)]">
            <Row label="Status">
              <Badge variant={round.status === "active" ? "blue" : "subtle"} size="sm">
                {STATUS_LABEL[round.status] ?? round.status}
              </Badge>
            </Row>
            <Row label="Time per team">{round.slot_duration_mins} min</Row>
            <Row label="Teams kept on deck per room">{round.buffer_size}</Row>
            <Row label="Created">{new Date(round.created_at).toLocaleString()}</Row>
          </div>
        </Section>

        <Section title={`Rooms${rooms ? ` (${rooms.length})` : ""}`}>
          {roomsLoading ? (
            <Skeleton className="h-16 w-full" />
          ) : !rooms || rooms.length === 0 ? (
            <p className="rounded-[14px] border border-dashed border-[var(--card-line)] p-4 text-center italic text-[var(--ink-tertiary)]">
              No rooms yet. Use Add room to create one.
            </p>
          ) : (
            <ul className="space-y-2">
              {rooms.map((room) => (
                <li key={room.panel_id} className="rounded-[14px] border border-[var(--card-line)] bg-[var(--card-solid)] p-3.5">
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-semibold text-[var(--ink-primary)]">
                      {room.name} <span className="font-normal text-[var(--ink-secondary)]">· {room.room}</span>
                    </span>
                    <span className="text-[var(--ink-secondary)] tabular-nums">
                      {room.judge_count} {room.judge_count === 1 ? "judge" : "judges"} · {room.slots.length}{" "}
                      {room.slots.length === 1 ? "team" : "teams"}
                    </span>
                  </div>
                  {room.slots.length > 0 && (
                    <ol className="mt-2 space-y-1 text-[var(--ink-secondary)]">
                      {room.slots.map((s) => (
                        <li key={s.slot_id} className="flex gap-2">
                          <span className="w-5 font-mono tabular-nums text-[var(--ink-tertiary)]">{s.sequence_order}</span>
                          <span className="text-[var(--ink-primary)]">{s.team_name}</span>
                        </li>
                      ))}
                    </ol>
                  )}
                </li>
              ))}
            </ul>
          )}
        </Section>

        <Section title="Scoring criteria">
          {isLoading || !roundDetail ? (
            <Skeleton className="h-16 w-full" />
          ) : roundDetail.criteria.length === 0 ? (
            <p className="italic text-[var(--ink-tertiary)]">No scoring criteria.</p>
          ) : (
            <div className="rounded-[14px] border border-[var(--card-line)] divide-y divide-[var(--card-line)] bg-[var(--card-solid)]">
              {roundDetail.criteria.map((c, idx) => (
                <div key={c.id || idx} className="flex items-start justify-between gap-3 px-3.5 py-2.5">
                  <div className="min-w-0">
                    <div className="font-semibold text-[var(--ink-primary)]">{c.name}</div>
                    {c.description && <div className="text-[var(--ink-secondary)] mt-0.5">{c.description}</div>}
                  </div>
                  <div className="shrink-0 text-right text-[var(--ink-secondary)] tabular-nums">
                    out of <strong className="text-[var(--ink-primary)]">{c.max_score}</strong>
                    <div>weight ×{c.weight}</div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </Section>
      </div>
    </Dialog>
  );
}
