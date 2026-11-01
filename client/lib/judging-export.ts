import { toCsv } from "@/lib/csv";
import type { AdminLeaderboardEntry, AdminLeaderboardResponse, ScoreSheetResponse } from "@/types/judging";

export const fmt = (n: number) => (Math.round(n * 100) / 100).toString();

/** Points a mark is worth: its position in the min..max range, times the criterion weight. */
export function weightedPoints(score: number, min: number, max: number, weight: number): number {
  return max > min ? ((score - min) / (max - min)) * weight : weight;
}

/** Best total first; teams with no locked scorecards go last, then by name. */
export function byTotal(a: AdminLeaderboardEntry, b: AdminLeaderboardEntry) {
  const aScored = a.judges_evaluated_count > 0;
  const bScored = b.judges_evaluated_count > 0;
  if (aScored !== bScored) return aScored ? -1 : 1;
  return b.total_weighted_score - a.total_weighted_score || a.team_name.localeCompare(b.team_name);
}

/**
 * One row per team: how the total is made up (judged + custom), each criterion's average, and the
 * custom criterion's marks. `scope` "panel" ranks within the given rows; "global" also carries overall rank.
 */
export function buildSummaryCsv(
  data: AdminLeaderboardResponse,
  entries: AdminLeaderboardEntry[],
  scope: "global" | "panel"
): string {
  const criteria = data.criteria;
  const judgedMax = criteria.reduce((n, c) => n + c.weight, 0);
  const header = [
    "Rank",
    ...(scope === "global" ? ["Overall rank"] : []),
    "Team",
    "Room",
    "Panel",
    "Status",
    "Judged score",
    "Judged max",
    "Custom criterion",
    "Custom marks",
    "Custom marks max",
    "Custom score",
    "Custom max",
    "Total score",
    "Max total",
    "Panel total",
    "Max panel total",
    "Judges locked",
    "Judges on panel",
    ...criteria.flatMap((c) => [`${c.name} (avg marks /${c.max_score})`, `${c.name} (points /${fmt(c.weight)})`]),
  ];
  let rank = 0;
  const rows = [...entries].sort(byTotal).map((e) => {
    const scored = e.judges_evaluated_count > 0;
    if (scored) rank += 1;
    const customMax = e.max_possible_score - judgedMax;
    return [
      scored ? rank : "",
      ...(scope === "global" ? [scored ? e.rank : ""] : []),
      e.team_name,
      e.room,
      e.panel_name,
      !scored ? "Not scored" : e.all_panel_judges_locked ? "Final" : "Provisional",
      scored ? fmt(e.judged_score) : "",
      fmt(judgedMax),
      e.custom_criterion ?? "",
      e.custom_awarded != null ? fmt(e.custom_awarded) : "",
      e.custom_max ? e.custom_max : "",
      e.custom_awarded != null ? fmt(e.custom_score) : "",
      fmt(customMax),
      scored ? fmt(e.total_weighted_score) : "",
      fmt(e.max_possible_score),
      scored ? fmt(e.total_weighted_score * e.judges_evaluated_count) : "",
      fmt(e.max_possible_score * e.total_judges_on_panel),
      e.judges_evaluated_count,
      e.total_judges_on_panel,
      ...criteria.flatMap((c) => [
        scored && c.name in e.criteria_averages ? fmt(e.criteria_averages[c.name]) : "",
        scored && e.criteria_weighted_scores && c.name in e.criteria_weighted_scores
          ? fmt(e.criteria_weighted_scores[c.name])
          : "",
      ]),
    ];
  });
  return toCsv(header, rows);
}

/** One row per mark: team x judge x criterion, with status and notes. Everything needed to audit a score. */
export function buildDetailedCsv(sheet: ScoreSheetResponse, teamIds?: Set<string>): string {
  const header = [
    "Team",
    "Room",
    "Panel",
    "Judge",
    "Criterion",
    "Type",
    "Marks",
    "Min",
    "Max",
    "Weight",
    "Points",
    "Status",
    "Notes",
    "Last updated",
  ];
  const rows = sheet.rows
    .filter((r) => !teamIds || teamIds.has(r.team_id))
    .map((r) => {
      const status = r.kind === "custom" ? "Awarded" : r.score === null ? "Pending" : r.locked ? "Locked" : "Draft";
      return [
        r.team_name,
        r.room,
        r.panel_name,
        r.kind === "custom" ? "Admin (awarded)" : r.judge_name ?? "",
        r.criterion_name,
        r.kind === "custom" ? "Custom" : "Judged",
        r.score ?? "",
        r.min_score,
        r.max_score,
        fmt(r.weight),
        r.score !== null ? fmt(weightedPoints(r.score, r.min_score, r.max_score, r.weight)) : "",
        status,
        r.notes ?? "",
        r.scored_at ?? "",
      ];
    });
  return toCsv(header, rows);
}
