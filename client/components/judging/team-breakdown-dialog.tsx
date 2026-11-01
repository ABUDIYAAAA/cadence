"use client";

import React, { useState } from "react";
import { Dialog } from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Download, Unlock } from "lucide-react";
import { Input } from "@/components/ui/input";
import { useToast } from "@/providers/toast-provider";
import { apiErrorMessage } from "@/lib/utils";
import { useScoreSheet, useUnlockScorecard } from "@/hooks/use-judging";
import { downloadCsv, slugify } from "@/lib/csv";
import { buildDetailedCsv, fmt, weightedPoints } from "@/lib/judging-export";
import type { AdminLeaderboardEntry, ScoreSheetRow } from "@/types/judging";

interface Props {
  roundId: string;
  team: AdminLeaderboardEntry | null;
  onClose: () => void;
}

/**
 * Exactly how one team's score was made: every judge's mark on every criterion (locked, draft or pending),
 * the admin-awarded custom criterion, and how those add up to the total shown on the leaderboard.
 */
export function TeamBreakdownDialog({ roundId, team, onClose }: Props) {
  const { data: sheet, isLoading } = useScoreSheet(roundId, Boolean(team));
  const unlock = useUnlockScorecard();
  const toast = useToast();
  // Who the admin is about to unlock ("all" = every locked judge on this team).
  const [confirm, setConfirm] = useState<{ judgeId: string; name: string } | null>(null);
  const [reason, setReason] = useState("");
  if (!team) return null;

  const rows = (sheet?.rows ?? []).filter((r) => r.team_id === team.team_id);
  const judged = rows.filter((r) => r.kind === "judged");
  const custom = rows.find((r) => r.kind === "custom");

  const judges = Array.from(new Map(judged.map((r): [string, string] => [r.judge_id ?? "", r.judge_name ?? ""])).entries()).sort((a, b) =>
    a[1].localeCompare(b[1])
  );
  const criteria = Array.from(
    new Map(judged.map((r) => [r.criterion_name, { name: r.criterion_name, min: r.min_score, max: r.max_score, weight: r.weight }])).values()
  );
  const cell = (judgeId: string, criterion: string): ScoreSheetRow | undefined =>
    judged.find((r) => r.judge_id === judgeId && r.criterion_name === criterion);

  const judgedMax = criteria.reduce((n, c) => n + c.weight, 0);
  const customMax = custom ? custom.weight : team.custom_weight ?? 0;
  const lockedJudges = judges.filter(([id]) => judged.some((r) => r.judge_id === id && r.locked)).length;
  const notes = judged.filter((r) => r.notes && r.notes.trim());

  const slotId = judged[0]?.slot_id;
  const lockedJudgeIds = judges.filter(([id]) => judged.some((r) => r.judge_id === id && r.locked)).map(([id]) => id);
  // An ended round is final, so its scorecards can't be reopened.
  const canUnlock = sheet?.status === "active" && Boolean(slotId);

  const runUnlock = async () => {
    if (!confirm || !slotId) return;
    try {
      const res = await unlock.mutateAsync({
        slotId,
        payload: { judge_id: confirm.judgeId === "all" ? undefined : confirm.judgeId, reason: reason.trim() || undefined },
      });
      toast.success(
        `Unlocked ${res.unlocked} ${res.unlocked === 1 ? "scorecard" : "scorecards"}${res.slot_reopened ? "; the team was reopened" : ""}.`
      );
      setConfirm(null);
      setReason("");
    } catch (err: unknown) {
      toast.error(apiErrorMessage(err, "Failed to unlock"));
    }
  };

  const exportTeam = () => {
    if (!sheet) return;
    downloadCsv(`${slugify(sheet.round_name)}-${slugify(team.team_name)}-scores.csv`, buildDetailedCsv(sheet, new Set([team.team_id])));
  };

  return (
    <Dialog
      open
      onClose={onClose}
      title={team.team_name}
      description={`${team.room} · ${team.panel_name}${team.judges_evaluated_count > 0 ? ` · rank ${team.rank}` : " · not ranked yet"}`}
      className="max-w-4xl"
      footer={
        <div className="flex justify-end gap-2">
          <Button variant="outline" size="sm" onClick={exportTeam} disabled={!sheet} className="gap-1.5">
            <Download className="h-3.5 w-3.5" />
            Export this team
          </Button>
          {canUnlock && lockedJudgeIds.length > 1 && (
            <Button variant="outline" size="sm" onClick={() => setConfirm({ judgeId: "all", name: "all judges" })} className="gap-1.5">
              <Unlock className="h-3.5 w-3.5" />
              Unlock all
            </Button>
          )}
          <Button variant="primary" size="sm" onClick={onClose}>
            Close
          </Button>
        </div>
      }
    >
      <div className="space-y-4 font-sans text-xs">
        {/* How the total is made */}
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
          <div className="rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-3">
            <div className="text-[var(--ink-secondary)]">Judged criteria</div>
            <div className="font-mono text-base font-bold tabular-nums text-[var(--ink-primary)]">
              {fmt(team.judged_score)} <span className="text-xs font-normal text-[var(--ink-tertiary)]">/ {fmt(judgedMax)}</span>
            </div>
            <div className="text-[11px] text-[var(--ink-tertiary)]">
              Average of {lockedJudges} locked {lockedJudges === 1 ? "judge" : "judges"}
            </div>
          </div>
          <div className="rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-3">
            <div className="text-[var(--ink-secondary)]">{team.custom_criterion || "Custom criterion"}</div>
            <div className="font-mono text-base font-bold tabular-nums text-[var(--ink-primary)]">
              {fmt(team.custom_score)} <span className="text-xs font-normal text-[var(--ink-tertiary)]">/ {fmt(customMax)}</span>
            </div>
            <div className="text-[11px] text-[var(--ink-tertiary)]">
              {custom && custom.score !== null
                ? `${fmt(custom.score)} of ${custom.max_score} marks, set by admin`
                : "No marks entered"}
            </div>
          </div>
          <div className="rounded-[12px] border border-[var(--color-blue)]/40 bg-[var(--color-blue-dim)] p-3">
            <div className="text-[var(--ink-secondary)]">Total</div>
            <div className="font-mono text-base font-bold tabular-nums text-[var(--ink-primary)]">
              {fmt(team.total_weighted_score)} <span className="text-xs font-normal text-[var(--ink-tertiary)]">/ {fmt(team.max_possible_score)}</span>
            </div>
            <div className="text-[11px] text-[var(--ink-tertiary)]">
              {fmt(team.judged_score)} + {fmt(team.custom_score)}
            </div>
          </div>
        </div>

        {confirm && (
          <div className="space-y-2 rounded-[12px] border border-[#CE6969]/30 bg-[var(--color-coral-dim)] p-3">
            <div className="font-semibold text-[var(--color-coral)]">
              Unlock {confirm.judgeId === "all" ? "every locked scorecard" : `${confirm.name}'s scorecard`} for {team.team_name}?
            </div>
            <p className="text-[var(--color-coral)]">
              {confirm.judgeId === "all" ? "Those judges" : "The judge"} can edit and must lock again. Their marks stop counting
              on the leaderboard until they do.
              {team.slot_status === "completed" && " This team has finished, so it will be reopened if no other team is presenting in its room."}
            </p>
            <Input
              placeholder="Reason (optional, saved in the audit log)"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              maxLength={500}
              disabled={unlock.isPending}
            />
            <div className="flex justify-end gap-1.5">
              <Button variant="outline" size="sm" onClick={() => setConfirm(null)} disabled={unlock.isPending}>
                Cancel
              </Button>
              <Button variant="destructive" size="sm" onClick={runUnlock} isLoading={unlock.isPending}>
                Unlock
              </Button>
            </div>
          </div>
        )}

        {isLoading ? (
          <div className="text-[var(--ink-tertiary)]">Loading marks...</div>
        ) : judged.length === 0 && !custom ? (
          <div className="rounded-[12px] border border-dashed border-[var(--card-line)] p-4 text-center italic text-[var(--ink-tertiary)]">
            No marks recorded for this team yet.
          </div>
        ) : (
          <div className="overflow-x-auto rounded-[12px] border border-[var(--card-line)]">
            <table className="w-full min-w-[34rem] border-collapse text-left">
              <thead>
                <tr className="border-b border-[var(--card-line)] bg-[var(--card-solid-subtle)] text-[var(--ink-secondary)]">
                  <th className="px-3 py-2 font-semibold">Criterion</th>
                  <th className="px-2 py-2 text-right font-semibold">Weight</th>
                  {judges.map(([id, name]) => (
                    <th key={id} className="px-2 py-2 text-right font-semibold">
                      <div>{name}</div>
                      {canUnlock && lockedJudgeIds.includes(id) && (
                        <button
                          type="button"
                          onClick={() => setConfirm({ judgeId: id, name })}
                          className="mt-0.5 inline-flex cursor-pointer items-center gap-1 text-[10px] font-medium text-[var(--color-blue)] hover:underline"
                          title={`Unlock ${name}'s scorecard`}
                        >
                          <Unlock className="h-3 w-3" />
                          Unlock
                        </button>
                      )}
                    </th>
                  ))}
                  <th className="px-2 py-2 text-right font-semibold">Average</th>
                  <th className="px-3 py-2 text-right font-semibold">Points</th>
                </tr>
              </thead>
              <tbody>
                {criteria.map((c) => {
                  const locked = judges
                    .map(([id]) => cell(id, c.name))
                    .filter((r): r is ScoreSheetRow => Boolean(r && r.locked && r.score !== null));
                  const avg = locked.length ? locked.reduce((n, r) => n + (r.score as number), 0) / locked.length : null;
                  return (
                    <tr key={c.name} className="border-b border-[var(--card-line)] last:border-0">
                      <td className="px-3 py-2 font-semibold text-[var(--ink-primary)]">
                        {c.name} <span className="font-normal text-[var(--ink-tertiary)]">/ {c.max}</span>
                      </td>
                      <td className="px-2 py-2 text-right font-mono tabular-nums text-[var(--ink-secondary)]">{fmt(c.weight)}</td>
                      {judges.map(([id]) => {
                        const r = cell(id, c.name);
                        return (
                          <td key={id} className="px-2 py-2 text-right font-mono tabular-nums">
                            {!r || r.score === null ? (
                              <span className="text-[var(--ink-tertiary)]">pending</span>
                            ) : r.locked ? (
                              <span className="font-semibold text-[var(--ink-primary)]">{fmt(r.score)}</span>
                            ) : (
                              <span className="text-[var(--ink-secondary)]" title="Draft: not locked, not counted">
                                {fmt(r.score)}<span className="text-[10px]"> draft</span>
                              </span>
                            )}
                          </td>
                        );
                      })}
                      <td className="px-2 py-2 text-right font-mono tabular-nums text-[var(--ink-primary)]">
                        {avg === null ? "—" : fmt(avg)}
                      </td>
                      <td className="px-3 py-2 text-right font-mono tabular-nums font-semibold text-[var(--ink-primary)]">
                        {avg === null ? "—" : fmt(weightedPoints(avg, c.min, c.max, c.weight))}
                      </td>
                    </tr>
                  );
                })}
                {custom && custom.score !== null && (
                  <tr className="bg-[var(--card-solid-subtle)]">
                    <td className="px-3 py-2 font-semibold text-[var(--ink-primary)]">
                      {custom.criterion_name} <span className="font-normal text-[var(--ink-tertiary)]">/ {custom.max_score}</span>{" "}
                      <Badge variant="subtle" size="sm">Custom</Badge>
                    </td>
                    <td className="px-2 py-2 text-right font-mono tabular-nums text-[var(--ink-secondary)]">{fmt(custom.weight)}</td>
                    <td colSpan={Math.max(judges.length, 1)} className="px-2 py-2 text-right text-[var(--ink-tertiary)]">
                      awarded by admin
                    </td>
                    <td className="px-2 py-2 text-right font-mono tabular-nums text-[var(--ink-primary)]">{fmt(custom.score)}</td>
                    <td className="px-3 py-2 text-right font-mono tabular-nums font-semibold text-[var(--ink-primary)]">
                      {fmt(weightedPoints(custom.score, custom.min_score, custom.max_score, custom.weight))}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        )}

        <p className="text-[11px] text-[var(--ink-tertiary)]">
          Only locked marks count. Points = (average marks − min) ÷ (max − min) × weight. Draft and pending marks are shown but not counted.
        </p>

        {notes.length > 0 && (
          <div className="space-y-1.5">
            <div className="font-semibold text-[var(--ink-secondary)]">Judge notes</div>
            {notes.map((r, i) => (
              <div key={i} className="rounded-[10px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-2">
                <span className="font-semibold text-[var(--ink-primary)]">{r.judge_name}</span>
                <span className="text-[var(--ink-tertiary)]"> on {r.criterion_name}: </span>
                <span className="text-[var(--ink-secondary)]">{r.notes}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </Dialog>
  );
}
