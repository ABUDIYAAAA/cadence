"use client";

import React, { useState } from "react";
import { useJudgingRounds, useAdminLeaderboard } from "@/hooks/use-judging";
import { SectionToolbar, MetricGrid, Metric } from "./section-shell";
import { Select } from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import { Download, FileSpreadsheet } from "lucide-react";
import { downloadCsv, slugify } from "@/lib/csv";
import { buildDetailedCsv, buildSummaryCsv, fmt } from "@/lib/judging-export";
import { judgingService } from "@/services/judging.service";
import { TeamBreakdownDialog } from "./team-breakdown-dialog";
import { useToast } from "@/providers/toast-provider";
import { apiErrorMessage } from "@/lib/utils";
import type { AdminLeaderboardEntry } from "@/types/judging";
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

/** Sum of every locked judge's score; the stored score is their mean, so mean x judges locked. */
const panelTotal = (e: { total_weighted_score: number; judges_evaluated_count: number }) =>
  e.total_weighted_score * e.judges_evaluated_count;

/**
 * Round standings from locked scorecards. Polls, so each judge's lock appears within seconds.
 * Teams with some but not all judges locked are shown as provisional.
 */
export function RoundLeaderboardView() {
  const { data: rounds = [], isLoading: roundsLoading } = useJudgingRounds();
  const [pickedRoundId, setPickedRoundId] = useState("");
  const [openTeam, setOpenTeam] = useState<AdminLeaderboardEntry | null>(null);
  const [exportingDetail, setExportingDetail] = useState(false);
  const toast = useToast();

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
    downloadCsv(`${slugify(data.round_name)}-all-teams.csv`, buildSummaryCsv(data, entries, "global"));
  };

  // Fetched fresh at click time so the file always matches what judges have entered right now.
  const exportDetailed = async () => {
    if (!data) return;
    setExportingDetail(true);
    try {
      const sheet = await judgingService.getScoreSheet(roundId);
      downloadCsv(`${slugify(data.round_name)}-all-marks.csv`, buildDetailedCsv(sheet));
    } catch (err) {
      toast.error(apiErrorMessage(err, "Failed to export marks"));
    } finally {
      setExportingDetail(false);
    }
  };

  const exportPanel = (key: string) => {
    if (!data || !key) return;
    const [room, name] = key.split("|");
    const rows = entries.filter((e) => e.room === room && e.panel_name === name);
    downloadCsv(`${slugify(data.round_name)}-${slugify(`${room}-${name}`)}.csv`, buildSummaryCsv(data, rows, "panel"));
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
          Export summary
        </Button>
        <Button
          variant="outline"
          size="sm"
          onClick={exportDetailed}
          isLoading={exportingDetail}
          disabled={!data || entries.length === 0}
          title="Every judge's mark on every criterion, with status and notes"
          className="flex items-center gap-1.5 text-xs h-9"
        >
          <FileSpreadsheet className="h-3.5 w-3.5" />
          Export all marks
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
                  <TableHead className="text-right">Judged</TableHead>
                  <TableHead className="text-right">Custom</TableHead>
                  <TableHead className="text-right">Total</TableHead>
                  <TableHead className="text-right">Panel total</TableHead>
                  <TableHead>Judges locked</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {entries.map((e) => {
                  const unscored = e.judges_evaluated_count === 0;
                  return (
                    <TableRow
                      key={e.team_id}
                      onClick={() => setOpenTeam(e)}
                      className="cursor-pointer hover:bg-[var(--card-solid-subtle)]"
                    >
                      <TableCell className="font-mono tabular-nums font-bold text-[var(--ink-primary)]">
                        {unscored ? <span className="text-[var(--ink-tertiary)]">—</span> : e.rank}
                      </TableCell>
                      <TableCell className="font-semibold text-[var(--ink-primary)]">
                        <button
                          type="button"
                          onClick={(ev) => {
                            ev.stopPropagation();
                            setOpenTeam(e);
                          }}
                          className="cursor-pointer text-left underline-offset-2 hover:underline focus-visible:underline"
                          title="See every judge's marks for this team"
                        >
                          {e.team_name}
                        </button>
                      </TableCell>
                      <TableCell className="text-[var(--ink-secondary)]">
                        {e.room} · {e.panel_name}
                      </TableCell>
                      <TableCell className="text-right font-mono tabular-nums text-[var(--ink-secondary)]">
                        {unscored ? "—" : fmt(e.judged_score)}
                      </TableCell>
                      <TableCell
                        className="text-right font-mono tabular-nums text-[var(--ink-secondary)]"
                        title={e.custom_criterion ? `${e.custom_criterion}: ${e.custom_awarded ?? "no"} marks` : "No custom criterion"}
                      >
                        {e.custom_awarded != null ? fmt(e.custom_score) : <span className="text-[var(--ink-tertiary)]">—</span>}
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

      <TeamBreakdownDialog roundId={roundId} team={openTeam} onClose={() => setOpenTeam(null)} />
    </div>
  );
}
