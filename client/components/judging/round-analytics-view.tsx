"use client";

import React, { useState } from "react";
import { useJudgingRounds, useRoundAnalytics } from "@/hooks/use-judging";
import { SectionToolbar, MetricGrid, Metric, Surface } from "./section-shell";
import { Select } from "@/components/ui/select";
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
import type { AdminLeaderboardEntry } from "@/types/judging";

const fmt = (n: number) => (Math.round(n * 100) / 100).toString();

/** 75 -> "1m 15s"; 0 -> "—" so unrecorded judges don't read as instant graders. */
function fmtDuration(secs: number): string {
  if (!secs || secs <= 0) return "—";
  const s = Math.round(secs);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ${s % 60}s`;
  return `${Math.floor(m / 60)}h ${m % 60}m`;
}

function Bar({ pct, className = "bg-[var(--color-blue)]" }: { pct: number; className?: string }) {
  const w = Math.max(0, Math.min(100, pct));
  return (
    <div className="h-1.5 w-full rounded-full bg-[var(--card-line)] overflow-hidden">
      <div className={`h-full rounded-full ${className}`} style={{ width: `${w}%` }} />
    </div>
  );
}

function TeamRows({ teams, max }: { teams: AdminLeaderboardEntry[]; max: number }) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead className="w-12">#</TableHead>
          <TableHead>Team</TableHead>
          <TableHead>Room</TableHead>
          <TableHead className="text-right">Score</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {teams.map((t) => (
          <TableRow key={t.team_id}>
            <TableCell className="font-mono font-bold text-[var(--ink-primary)]">{t.rank}</TableCell>
            <TableCell className="font-semibold text-[var(--ink-primary)]">{t.team_name}</TableCell>
            <TableCell className="text-[var(--ink-secondary)]">
              {t.room} · {t.panel_name}
            </TableCell>
            <TableCell className="text-right font-mono tabular-nums">
              <strong className="text-[var(--ink-primary)]">{fmt(t.total_weighted_score)}</strong>
              <span className="text-[var(--ink-tertiary)]"> / {fmt(max)}</span>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}

/** Admin analytics for a round: overall scores, panel comparison, rubric and judge behaviour. */
export function RoundAnalyticsView() {
  const { data: rounds = [], isLoading: roundsLoading } = useJudgingRounds();
  const [pickedRoundId, setPickedRoundId] = useState("");

  const defaultRound = rounds.find((r) => r.status === "active") ?? rounds[0];
  const roundId = pickedRoundId || defaultRound?.id || "";

  const { data, isLoading, isFetching, refetch } = useRoundAnalytics(roundId);

  if (roundsLoading) return <SkeletonCard />;

  if (rounds.length === 0) {
    return (
      <EmptyState title="No rounds yet" description="Create a judging round to see analytics here." />
    );
  }

  return (
    <div className="font-sans space-y-6">
      <SectionToolbar
        title="Analytics"
        subtitle="Scores, panels, rubric and judge behaviour from locked scorecards"
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
          <MetricGrid>
            <Metric
              label="Fully scored"
              value={data.summary.graded_teams}
              suffix={`/ ${data.summary.total_teams} teams`}
              note={data.summary.partial_teams > 0 ? `${data.summary.partial_teams} in progress` : undefined}
              valueClassName="text-[var(--color-blue)]"
            />
            <Metric label="Total score" value={fmt(data.summary.total_score)} note="Sum across scored teams" />
            <Metric
              label="Average score"
              value={fmt(data.summary.average_score)}
              suffix={`/ ${fmt(data.summary.max_possible_score)}`}
              note={`Median ${fmt(data.summary.median_score)}`}
            />
            <Metric
              label="Score range"
              value={`${fmt(data.summary.lowest_score)}–${fmt(data.summary.highest_score)}`}
              note="Lowest to highest"
            />
            <Metric label="Judges" value={data.summary.total_judges} />
            <Metric label="Scorecards locked" value={data.summary.scorecards_locked} />
            <Metric label="Avg grading time" value={fmtDuration(data.summary.avg_grading_secs)} note="Per scorecard" />
            <Metric label="Round status" value={data.status} />
          </MetricGrid>

          <div className="grid gap-4 lg:grid-cols-2">
            <Surface>
              <div className="text-sm font-semibold text-[var(--ink-primary)]">Top teams overall</div>
              {data.top_teams.length === 0 ? (
                <div className="text-xs text-[var(--ink-secondary)]">
                  Teams appear once every judge on their panel has locked.
                </div>
              ) : (
                <TeamRows teams={data.top_teams} max={data.summary.max_possible_score} />
              )}
            </Surface>

            <Surface>
              <div className="text-sm font-semibold text-[var(--ink-primary)]">Score distribution</div>
              {(() => {
                const peak = Math.max(1, ...data.score_distribution.map((b) => b.count));
                return (
                  <div className="space-y-2.5">
                    {data.score_distribution.map((b) => (
                      <div key={b.label} className="grid grid-cols-[64px_1fr_28px] items-center gap-3 text-xs">
                        <span className="text-[var(--ink-secondary)]">{b.label}</span>
                        <Bar pct={(b.count / peak) * 100} />
                        <span className="font-mono text-right text-[var(--ink-primary)]">{b.count}</span>
                      </div>
                    ))}
                    <div className="text-[11px] text-[var(--ink-tertiary)]">Share of the maximum possible score</div>
                  </div>
                );
              })()}
              {data.bottom_teams.length > 0 && (
                <>
                  <div className="text-sm font-semibold text-[var(--ink-primary)] pt-2">Lowest scoring</div>
                  <TeamRows teams={data.bottom_teams} max={data.summary.max_possible_score} />
                </>
              )}
            </Surface>
          </div>

          <Surface>
            <div className="text-sm font-semibold text-[var(--ink-primary)]">Panels</div>
            {data.panels.length === 0 ? (
              <div className="text-xs text-[var(--ink-secondary)]">No panels in this round.</div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Panel</TableHead>
                    <TableHead className="text-right">Scored</TableHead>
                    <TableHead className="text-right">Total</TableHead>
                    <TableHead className="text-right">Average</TableHead>
                    <TableHead className="text-right">High / Low</TableHead>
                    <TableHead>Top team</TableHead>
                    <TableHead className="text-right">Avg grading</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {data.panels.map((p) => (
                    <TableRow key={p.panel_id}>
                      <TableCell className="font-semibold text-[var(--ink-primary)]">
                        {p.room} · {p.panel_name}
                        <div className="text-[11px] font-normal text-[var(--ink-tertiary)]">
                          {p.judges} judge{p.judges === 1 ? "" : "s"}
                        </div>
                      </TableCell>
                      <TableCell className="text-right font-mono tabular-nums">
                        {p.graded_teams}/{p.total_teams}
                      </TableCell>
                      <TableCell className="text-right font-mono tabular-nums">{fmt(p.total_score)}</TableCell>
                      <TableCell className="text-right font-mono tabular-nums">
                        {p.graded_teams ? fmt(p.average_score) : "—"}
                      </TableCell>
                      <TableCell className="text-right font-mono tabular-nums text-[var(--ink-secondary)]">
                        {p.graded_teams ? `${fmt(p.highest_score)} / ${fmt(p.lowest_score)}` : "—"}
                      </TableCell>
                      <TableCell>{p.top_team || "—"}</TableCell>
                      <TableCell className="text-right font-mono tabular-nums">
                        {fmtDuration(p.avg_grading_secs)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </Surface>

          <Surface>
            <div className="text-sm font-semibold text-[var(--ink-primary)]">Criteria</div>
            <div className="space-y-3">
              {data.criteria.map((c) => (
                <div key={c.criterion_id} className="space-y-1">
                  <div className="flex items-baseline justify-between gap-3 text-xs">
                    <span className="font-semibold text-[var(--ink-primary)]">
                      {c.name}
                      <span className="font-normal text-[var(--ink-tertiary)]"> · weight {fmt(c.weight)}</span>
                    </span>
                    <span className="font-mono text-[var(--ink-secondary)]">
                      avg {fmt(c.avg_score)} / {c.max_score}
                      {c.scores > 0 && ` · range ${fmt(c.lowest_score)}–${fmt(c.highest_score)}`}
                    </span>
                  </div>
                  <Bar pct={c.avg_pct * 100} />
                </div>
              ))}
            </div>
          </Surface>

          <Surface>
            <div className="text-sm font-semibold text-[var(--ink-primary)]">Judges</div>
            {data.judges.length === 0 ? (
              <div className="text-xs text-[var(--ink-secondary)]">No judges assigned yet.</div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Judge</TableHead>
                    <TableHead className="text-right">Teams scored</TableHead>
                    <TableHead className="text-right">Avg grading</TableHead>
                    <TableHead className="text-right">Fastest / slowest</TableHead>
                    <TableHead className="text-right">Avg edits</TableHead>
                    <TableHead className="text-right">Total given</TableHead>
                    <TableHead className="text-right">Avg / team</TableHead>
                    <TableHead className="text-right">Avg score</TableHead>
                    <TableHead>Per criterion (avg)</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {data.judges.map((j) => (
                    <TableRow key={j.judge_id}>
                      <TableCell className="font-semibold text-[var(--ink-primary)]">
                        {j.name}
                        <div className="text-[11px] font-normal text-[var(--ink-tertiary)]">{j.panels.join(", ")}</div>
                      </TableCell>
                      <TableCell className="text-right font-mono tabular-nums">
                        {j.scorecards_locked}/{j.scorecards_assigned}
                      </TableCell>
                      <TableCell className="text-right font-mono tabular-nums">{fmtDuration(j.avg_grading_secs)}</TableCell>
                      <TableCell className="text-right font-mono tabular-nums text-[var(--ink-secondary)]">
                        {j.scorecards_locked
                          ? `${fmtDuration(j.min_grading_secs)} / ${fmtDuration(j.max_grading_secs)}`
                          : "—"}
                      </TableCell>
                      <TableCell className="text-right font-mono tabular-nums">
                        {j.scorecards_locked ? fmt(j.avg_edits) : "—"}
                      </TableCell>
                      <TableCell className="text-right font-mono tabular-nums">{fmt(j.total_score_given)}</TableCell>
                      <TableCell className="text-right font-mono tabular-nums">
                        {j.scorecards_locked ? fmt(j.avg_total_per_team) : "—"}
                      </TableCell>
                      <TableCell className="text-right font-mono tabular-nums">
                        {j.scorecards_locked ? `${fmt(j.avg_score)} (${Math.round(j.avg_pct * 100)}%)` : "—"}
                      </TableCell>
                      <TableCell className="text-xs text-[var(--ink-secondary)]">
                        {j.criteria.length === 0
                          ? "—"
                          : j.criteria.map((c) => `${c.name} ${fmt(c.avg_score)}`).join(" · ")}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
            <div className="text-[11px] text-[var(--ink-tertiary)]">
              Grading time runs from the start of a team&apos;s pitch to the judge locking their scorecard, excluding
              paused time. Edits count score changes saved after the first save.
            </div>
          </Surface>
        </>
      )}
    </div>
  );
}
