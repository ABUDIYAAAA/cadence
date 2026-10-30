"use client";

import React, { useState } from "react";
import { useJudgingRounds, useAdminLeaderboard } from "@/hooks/use-judging";
import { SectionToolbar, MetricGrid, Metric } from "./section-shell";
import { Select } from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import { Download } from "lucide-react";
import { downloadCsv, slugify, toCsv } from "@/lib/csv";
import type { AdminLeaderboardEntry, AdminLeaderboardResponse } from "@/types/judging";
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

/** Sum of every locked judge's score; the stored score is their mean, so mean x judges locked. */
const panelTotal = (e: { total_weighted_score: number; judges_evaluated_count: number }) =>
  e.total_weighted_score * e.judges_evaluated_count;

/** Best average first; teams with no locked scorecards go last, then by name. */
function byAverage(a: AdminLeaderboardEntry, b: AdminLeaderboardEntry) {
  const aScored = a.judges_evaluated_count > 0;
  const bScored = b.judges_evaluated_count > 0;
  if (aScored !== bScored) return aScored ? -1 : 1;
  return b.total_weighted_score - a.total_weighted_score || a.team_name.localeCompare(b.team_name);
}

function buildCsv(data: AdminLeaderboardResponse, entries: AdminLeaderboardEntry[], scope: "global" | "panel") {
  const criteria = data.criteria.map((c) => c.name);
  const header = [
    "Rank",
    ...(scope === "global" ? ["Overall rank"] : []),
    "Team",
    "Room",
    "Panel",
    "Avg score",
    "Max score",
    "Panel total",
    "Max panel total",
    "Judges locked",
    "Judges on panel",
    "Status",
    ...criteria.map((n) => `${n} (avg)`),
  ];
  let rank = 0;
  const rows = [...entries].sort(byAverage).map((e) => {
    const scored = e.judges_evaluated_count > 0;
    if (scored) rank += 1;
    return [
      scored ? rank : "",
      ...(scope === "global" ? [scored ? e.rank : ""] : []),
      e.team_name,
      e.room,
      e.panel_name,
      scored ? fmt(e.total_weighted_score) : "",
      fmt(e.max_possible_score),
      scored ? fmt(panelTotal(e)) : "",
      fmt(e.max_possible_score * e.total_judges_on_panel),
      e.judges_evaluated_count,
      e.total_judges_on_panel,
      !scored ? "Not scored" : e.all_panel_judges_locked ? "Final" : "Provisional",
      ...criteria.map((n) => (scored && n in e.criteria_averages ? fmt(e.criteria_averages[n]) : "")),
    ];
  });
  return toCsv(header, rows);
}

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

  const panels = Array.from(
    new Map(entries.map((e) => [`${e.room}|${e.panel_name}`, { room: e.room, name: e.panel_name }])).values()
  );

  const exportGlobal = () => {
    if (!data) return;
    downloadCsv(`${slugify(data.round_name)}-all-teams.csv`, buildCsv(data, entries, "global"));
  };

  const exportPanel = (key: string) => {
    if (!data || !key) return;
    const [room, name] = key.split("|");
    const rows = entries.filter((e) => e.room === room && e.panel_name === name);
    downloadCsv(`${slugify(data.round_name)}-${slugify(`${room}-${name}`)}.csv`, buildCsv(data, rows, "panel"));
  };

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
        <Button
          variant="outline"
          size="sm"
          onClick={exportGlobal}
          disabled={!data || entries.length === 0}
          className="flex items-center gap-1.5 text-xs h-9"
        >
          <Download className="h-3.5 w-3.5" />
          Export all
        </Button>
        {panels.length > 0 && (
          <div className="w-44">
            <Select
              aria-label="Export a single panel"
              value=""
              onChange={(e) => exportPanel(e.target.value)}
              className="text-xs h-9"
            >
              <option value="">Export panel…</option>
              {panels.map((p) => (
                <option key={`${p.room}|${p.name}`} value={`${p.room}|${p.name}`}>
                  {p.room} · {p.name}
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
                  <TableHead className="text-right">Avg score</TableHead>
                  <TableHead className="text-right">Panel total</TableHead>
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
                      <TableCell className="text-right font-mono tabular-nums">
                        {unscored ? (
                          <span className="text-[var(--ink-tertiary)]">—</span>
                        ) : (
                          <>
                            <strong className="text-[var(--ink-primary)]">{fmt(panelTotal(e))}</strong>
                            <span className="text-[var(--ink-tertiary)]">
                              {" "}
                              / {fmt(e.max_possible_score * e.total_judges_on_panel)}
                            </span>
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
