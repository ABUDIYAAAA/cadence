"use client";

import React, { useState } from "react";
import { useJudgingRounds, useAdminLeaderboard } from "@/hooks/use-judging";
import { SectionToolbar, MetricGrid, Metric } from "./section-shell";
import { Select } from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { SkeletonCard } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

const fmt = (n: number) => (Math.round(n * 100) / 100).toString();

/**
 * Round standings from locked scorecards. Polls, so each judge's lock appears within seconds.
 * Teams with some but not all judges locked are shown as provisional.
 */
export function RoundLeaderboardView() {
  const { data: rounds = [], isLoading: roundsLoading } = useJudgingRounds();
  const [pickedRoundId, setPickedRoundId] = useState("");

  // Default to the live round, else the newest
  const defaultRound = rounds.find((r) => r.status === "active") ?? rounds[0];
  const roundId = pickedRoundId || defaultRound?.id || "";

  const { data, isLoading, isFetching, refetch } = useAdminLeaderboard(roundId);

  if (roundsLoading) return <SkeletonCard />;

  if (rounds.length === 0) {
    return (
      <EmptyState
        title="No rounds yet"
        description="Create a judging round to see standings here."
      />
    );
  }

  const entries = data?.leaderboard ?? [];

  return (
    <div className="font-sans space-y-6">
      <SectionToolbar
        title="Leaderboard"
        subtitle="Ranked by locked scorecards; updates as judges lock their scores"
        onRefresh={() => refetch()}
        isFetching={isFetching}
      >
        {rounds.length > 1 && (
          <div className="w-52">
            <Select
              aria-label="Round"
              value={roundId}
              onChange={(e) => setPickedRoundId(e.target.value)}
              className="text-xs h-9"
            >
              {rounds.map((r) => (
                <option key={r.id} value={r.id}>
                  {r.name}
                </option>
              ))}
            </Select>
          </div>
        )}
      </SectionToolbar>

      {isLoading || !data ? (
        <SkeletonCard />
      ) : (
        <>
          <MetricGrid cols={3}>
            <Metric
              label="Fully scored"
              value={data.graded_teams}
              suffix={`/ ${data.total_teams} teams`}
              valueClassName="text-[var(--color-blue)]"
            />
            <Metric label="Average score" value={fmt(data.average_score)} suffix={`/ ${fmt(data.max_possible_score)}`} />
            <Metric label="Round status" value={data.status} />
          </MetricGrid>

          {entries.length === 0 ? (
            <EmptyState
              title="No teams scheduled"
              description="Teams appear here once they are assigned to a room."
            />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-14">Rank</TableHead>
                  <TableHead>Team</TableHead>
                  <TableHead>Room</TableHead>
                  <TableHead className="text-right">Score</TableHead>
                  <TableHead>Judges locked</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {entries.map((e) => {
                  const unscored = e.judges_evaluated_count === 0;
                  return (
                    <TableRow key={e.team_id}>
                      <TableCell className="font-mono tabular-nums font-bold text-[var(--ink-primary)]">
                        {unscored ? <span className="text-[var(--ink-tertiary)]">—</span> : e.rank}
                      </TableCell>
                      <TableCell className="font-semibold text-[var(--ink-primary)]">{e.team_name}</TableCell>
                      <TableCell className="text-[var(--ink-secondary)]">
                        {e.room} · {e.panel_name}
                      </TableCell>
                      <TableCell className="text-right font-mono tabular-nums">
                        {unscored ? (
                          <span className="text-[var(--ink-tertiary)]">—</span>
                        ) : (
                          <>
                            <strong className="text-[var(--ink-primary)]">{fmt(e.total_weighted_score)}</strong>
                            <span className="text-[var(--ink-tertiary)]"> / {fmt(e.max_possible_score)}</span>
                          </>
                        )}
                      </TableCell>
                      <TableCell>
                        {e.all_panel_judges_locked ? (
                          <Badge variant="emerald" size="sm">
                            {e.judges_evaluated_count} of {e.total_judges_on_panel}
                          </Badge>
                        ) : (
                          <Badge variant="subtle" size="sm">
                            {e.judges_evaluated_count} of {e.total_judges_on_panel}
                            {!unscored && " · provisional"}
                          </Badge>
                        )}
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          )}
        </>
      )}
    </div>
  );
}
