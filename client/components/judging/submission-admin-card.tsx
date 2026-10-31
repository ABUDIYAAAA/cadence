"use client";

import React, { useState } from "react";
import { Download } from "lucide-react";
import {
  useJudgingRounds,
  useSubmissionGate,
  useSetSubmissionGate,
} from "@/hooks/use-judging";
import { useScanSessions } from "@/hooks/use-tickets";
import { judgingService } from "@/services/judging.service";
import { useToast } from "@/providers/toast-provider";
import { apiErrorMessage } from "@/lib/utils";
import { downloadCsv, slugify, toCsv } from "@/lib/csv";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";

/**
 * Admin controls for project submissions: pick the attendance session a team must have been
 * present at (>=1 member scanned) to submit, and export every team's project name + repo link.
 */
export function SubmissionAdminCard() {
  const toast = useToast();
  const { data: gate } = useSubmissionGate();
  const setGate = useSetSubmissionGate();
  const { data: sessions = [] } = useScanSessions();
  const { data: rounds = [] } = useJudgingRounds();
  const [pickedRound, setPickedRound] = useState("");
  const [exporting, setExporting] = useState(false);

  const attendance = sessions.filter((s) => s.kind === "attendance");
  const defaultRound = rounds.find((r) => r.status === "active") ?? rounds[0];
  const roundId = pickedRound || defaultRound?.id || "";

  const changeGate = async (value: string) => {
    try {
      await setGate.mutateAsync(value || null);
      toast.success(value ? "Submission gate updated." : "Submission gate removed.");
    } catch (err: unknown) {
      toast.error(apiErrorMessage(err, "Could not update the submission gate."));
    }
  };

  const exportSubmissions = async () => {
    if (!roundId) return;
    setExporting(true);
    try {
      const data = await judgingService.getRoundSubmissions(roundId);
      const rows = data.submissions.map((r) => [r.team_name, r.project_name, r.repository]);
      downloadCsv(
        `${slugify(data.round_name)}-submissions.csv`,
        toCsv(["Team", "Project name", "Repository link"], rows)
      );
    } catch (err: unknown) {
      toast.error(apiErrorMessage(err, "Could not export submissions."));
    } finally {
      setExporting(false);
    }
  };

  return (
    <div className="bg-[var(--card-solid)] border border-[var(--card-line)] rounded-[16px] p-4 sm:p-5 flex flex-col md:flex-row md:items-end gap-4">
      <div className="flex-1 space-y-1">
        <Select
          label="Submission gate (attendance session)"
          value={gate?.session_id ?? ""}
          onChange={(e) => changeGate(e.target.value)}
          disabled={setGate.isPending}
        >
          <option value="">No gate: every team can submit</option>
          {attendance.map((s) => (
            <option key={s.id} value={s.id}>
              {s.name}
            </option>
          ))}
        </Select>
        <p className="text-[11px] text-[var(--ink-secondary)]">
          Teams can submit only if at least one member was scanned in this session.
        </p>
      </div>

      <div className="flex items-end gap-2">
        {rounds.length > 1 && (
          <div className="w-44">
            <Select
              label="Round"
              value={roundId}
              onChange={(e) => setPickedRound(e.target.value)}
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
          onClick={exportSubmissions}
          disabled={exporting || !roundId}
          className="text-xs h-9"
        >
          <Download className="h-3.5 w-3.5" />
          {exporting ? "Exporting…" : "Export submissions"}
        </Button>
      </div>
    </div>
  );
}
