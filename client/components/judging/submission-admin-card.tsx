"use client";

import React, { useState, useMemo } from "react";
import { Download, ExternalLink, Eye, Search } from "lucide-react";
import {
  useJudgingRounds,
  useSubmissionGate,
  useSetSubmissionGate,
  useSubmissions,
} from "@/hooks/use-judging";
import { useScanSessions } from "@/hooks/use-tickets";
import { judgingService } from "@/services/judging.service";
import { useToast } from "@/providers/toast-provider";
import { formatSubmissionsCsv } from "@/lib/judging-utils";
import { apiErrorMessage } from "@/lib/utils";
import { downloadCsv } from "@/lib/csv";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import { Dialog } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";

/**
 * Admin controls for project submissions: pick the attendance session a team must have been
 * present at (>=1 member scanned) to submit, inspect submissions in-app, and export all 5 columns as CSV.
 */
export function SubmissionAdminCard() {
  const toast = useToast();
  const { data: rounds = [] } = useJudgingRounds();
  const [selectedRoundId, setSelectedRoundId] = useState<string>("");

  const effectiveRoundId = useMemo(() => {
    if (selectedRoundId && rounds.some((r) => r.id === selectedRoundId)) {
      return selectedRoundId;
    }
    const active = rounds.find((r) => r.status === "active");
    if (active) return active.id;
    return rounds[0]?.id ?? "";
  }, [selectedRoundId, rounds]);

  const { data: gate } = useSubmissionGate(effectiveRoundId || null);
  const setGate = useSetSubmissionGate();
  const { data: sessions = [] } = useScanSessions();
  const [exporting, setExporting] = useState(false);
  const [viewerOpen, setViewerOpen] = useState(false);
  const [search, setSearch] = useState("");

  const {
    data: submissionsData,
    isLoading: submissionsLoading,
    refetch: refetchSubmissions,
  } = useSubmissions({ enabled: viewerOpen });

  const eligibleSessions = sessions.filter(
    (s) => s.kind === "attendance" || s.kind === "check_in"
  );

  const submissions = useMemo(
    () => submissionsData?.submissions ?? [],
    [submissionsData]
  );

  const submittedCount = useMemo(
    () =>
      submissions.filter(
        (r) =>
          Boolean(r.project_name?.trim()) ||
          Boolean(r.repository?.trim()) ||
          Boolean(r.demo?.trim()) ||
          Boolean(r.slides?.trim())
      ).length,
    [submissions]
  );

  const filteredSubmissions = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return submissions;
    return submissions.filter(
      (r) =>
        r.team_name.toLowerCase().includes(q) ||
        (r.project_name && r.project_name.toLowerCase().includes(q)) ||
        (r.repository && r.repository.toLowerCase().includes(q))
    );
  }, [submissions, search]);

  const changeGate = async (value: string) => {
    try {
      await setGate.mutateAsync({
        roundId: effectiveRoundId || null,
        sessionId: value || null,
      });
      toast.success(
        value
          ? "Submission gate updated for this round."
          : "Gate cleared — submissions are open immediately to all teams."
      );
    } catch (err: unknown) {
      toast.error(apiErrorMessage(err, "Could not update the submission gate."));
    }
  };

  const exportSubmissions = async () => {
    setExporting(true);
    try {
      const data = await judgingService.listSubmissions();
      downloadCsv("submissions.csv", formatSubmissionsCsv(data.submissions));
    } catch (err: unknown) {
      toast.error(apiErrorMessage(err, "Could not export submissions."));
    } finally {
      setExporting(false);
    }
  };

  return (
    <>
      <div className="bg-[var(--card-solid)] border border-[var(--card-line)] rounded-[16px] p-4 sm:p-5 flex flex-col lg:flex-row lg:items-end gap-4">
        <div className="flex-1 space-y-1.5">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {rounds.length > 0 && (
              <Select
                label="Judging round"
                value={effectiveRoundId}
                onChange={(e) => setSelectedRoundId(e.target.value)}
                disabled={setGate.isPending}
              >
                {rounds.map((r) => (
                  <option key={r.id} value={r.id}>
                    {r.name} ({r.status})
                  </option>
                ))}
              </Select>
            )}

            <Select
              label="Submission gate (optional)"
              value={gate?.session_id ?? ""}
              onChange={(e) => changeGate(e.target.value)}
              disabled={setGate.isPending}
            >
              <option value="">No gate — open to all teams immediately</option>
              {eligibleSessions.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name} ({s.kind === "check_in" ? "Entry check-in" : "Attendance"})
                </option>
              ))}
            </Select>
          </div>
          <p className="text-[11px] text-[var(--ink-secondary)]">
            {gate?.session_id
              ? "Only teams with at least one member scanned in this session can submit or edit their project links."
              : "No gate selected — submissions are open immediately to all teams."}
          </p>
        </div>

        <div className="flex items-end gap-2 flex-wrap">
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              setViewerOpen(true);
              refetchSubmissions();
            }}
            className="text-xs h-9 gap-1.5"
          >
            <Eye className="h-3.5 w-3.5" />
            View submissions
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={exportSubmissions}
            disabled={exporting}
            className="text-xs h-9 gap-1.5"
          >
            <Download className="h-3.5 w-3.5" />
            {exporting ? "Exporting…" : "Export CSV"}
          </Button>
        </div>
      </div>

      <Dialog
        open={viewerOpen}
        onClose={() => setViewerOpen(false)}
        title="Team submissions"
        description="Project names and repository, demo, and slide links submitted across this competition."
        className="max-w-4xl"
        footer={
          <div className="flex items-center justify-between w-full">
            <span className="text-xs text-[var(--ink-secondary)]">
              {submittedCount} of {submissions.length} teams have submitted
            </span>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={exportSubmissions}
                disabled={exporting}
                className="text-xs gap-1.5"
              >
                <Download className="h-3.5 w-3.5" />
                {exporting ? "Exporting…" : "Export CSV"}
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setViewerOpen(false)}
              >
                Close
              </Button>
            </div>
          </div>
        }
      >
        <div className="space-y-3 font-sans text-xs">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-[var(--ink-tertiary)]" />
            <Input
              placeholder="Search by team, project name, or repository..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-8 text-xs h-9"
            />
          </div>

          <div className="max-h-[55vh] overflow-y-auto rounded-[12px] border border-[var(--card-line)]">
            {submissionsLoading ? (
              <div className="p-6 text-center text-[var(--ink-tertiary)]">
                Loading submissions...
              </div>
            ) : filteredSubmissions.length === 0 ? (
              <div className="p-6 text-center text-[var(--ink-tertiary)]">
                No teams match your search.
              </div>
            ) : (
              <table className="w-full text-left border-collapse">
                <thead>
                  <tr className="border-b border-[var(--card-line)] bg-[var(--card-solid-subtle)] text-[11px] font-semibold text-[var(--ink-secondary)]">
                    <th className="py-2.5 px-3">Team</th>
                    <th className="py-2.5 px-3">Project</th>
                    <th className="py-2.5 px-3">Links</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--card-line)]">
                  {filteredSubmissions.map((row) => {
                    const links = [
                      { label: "Repo", url: row.repository },
                      { label: "Demo", url: row.demo },
                      { label: "Slides", url: row.slides },
                    ].filter((l) => Boolean(l.url && l.url.trim()));

                    return (
                      <tr key={row.team_name} className="hover:bg-[var(--card-solid-subtle)]/50">
                        <td className="py-2.5 px-3 font-semibold text-[var(--ink-primary)]">
                          {row.team_name}
                        </td>
                        <td className="py-2.5 px-3 text-[var(--ink-primary)]">
                          {row.project_name ? (
                            row.project_name
                          ) : (
                            <span className="text-[var(--ink-tertiary)] italic">
                              Not set
                            </span>
                          )}
                        </td>
                        <td className="py-2.5 px-3">
                          {links.length === 0 ? (
                            <Badge variant="subtle" size="sm">
                              No links
                            </Badge>
                          ) : (
                            <div className="flex flex-wrap items-center gap-1.5">
                              {links.map((l) => (
                                <a
                                  key={l.label}
                                  href={l.url}
                                  target="_blank"
                                  rel="noreferrer"
                                  className="inline-flex items-center gap-1 rounded-[6px] border border-[var(--card-line)] bg-[var(--card-solid)] px-2 py-0.5 text-[11px] font-medium text-[var(--ink-primary)] hover:border-[var(--color-blue)] hover:text-[var(--color-blue)] transition-colors"
                                >
                                  <span>{l.label}</span>
                                  <ExternalLink className="h-2.5 w-2.5" />
                                </a>
                              ))}
                            </div>
                          )}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            )}
          </div>
        </div>
      </Dialog>
    </>
  );
}
