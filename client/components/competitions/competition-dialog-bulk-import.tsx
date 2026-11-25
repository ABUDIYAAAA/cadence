"use client";

import React, { useMemo, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  AlertCircle,
  CheckCircle2,
  Upload,
  UserPlus,
} from "lucide-react";
import { eventsService } from "@/services/events.service";
import { useToast } from "@/providers/toast-provider";
import { apiErrorMessage } from "@/lib/utils";
import type { Competition } from "@/types/competitions";
import type { MissingUserEntry } from "@/types/events";
import type { BulkCreateResponse } from "@/types/users";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Select } from "@/components/ui/select";

interface CompetitionBulkImportDialogProps {
  open: boolean;
  onClose: () => void;
  eventSlug: string;
  competitions: ReadonlyArray<Competition>;
  initialCompetitionSlug?: string | null;
}

interface ParsedRow {
  email: string;
  team_name: string;
  name?: string;
  phone_number?: string;
}

const SAMPLE_CSV = [
  "email,team_name",
  "alice@example.com,Neural Ninjas",
  "bob@example.com,Neural Ninjas",
  "charlie@example.com,Pixel Forge",
].join("\n");

/** Splits a single CSV line, respecting double-quoted cells and escaped quotes (""). */
function parseCsvLine(line: string): string[] {
  const cells: string[] = [];
  let current = "";
  let inQuotes = false;

  for (let i = 0; i < line.length; i++) {
    const ch = line[i];
    if (inQuotes) {
      if (ch === '"') {
        if (line[i + 1] === '"') {
          current += '"';
          i++;
        } else {
          inQuotes = false;
        }
      } else {
        current += ch;
      }
    } else if (ch === '"') {
      inQuotes = true;
    } else if (ch === ",") {
      cells.push(current.trim());
      current = "";
    } else {
      current += ch;
    }
  }
  cells.push(current.trim());
  return cells;
}

function findColumn(header: string[], aliases: string[]): number {
  for (const alias of aliases) {
    const idx = header.indexOf(alias);
    if (idx !== -1) return idx;
  }
  return -1;
}

export function CompetitionBulkImportDialog({
  open,
  onClose,
  eventSlug,
  competitions,
  initialCompetitionSlug,
}: CompetitionBulkImportDialogProps) {
  const toast = useToast();
  const queryClient = useQueryClient();

  const [selectedSlug, setSelectedSlug] = useState<string>(
    initialCompetitionSlug || competitions[0]?.slug || ""
  );
  const [csvText, setCsvText] = useState("");
  const [parsedRows, setParsedRows] = useState<ParsedRow[]>([]);
  const [parseError, setParseError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  // Pass 1 (skip missing) & Pass 2 (create missing) results
  const [pass1Result, setPass1Result] = useState<BulkCreateResponse | null>(
    null
  );
  const [pass2Result, setPass2Result] = useState<BulkCreateResponse | null>(
    null
  );

  // Sync default competition when dialog opens for a specific row
  const effectiveSlug = useMemo(() => {
    if (selectedSlug && competitions.some((c) => c.slug === selectedSlug)) {
      return selectedSlug;
    }
    if (
      initialCompetitionSlug &&
      competitions.some((c) => c.slug === initialCompetitionSlug)
    ) {
      return initialCompetitionSlug;
    }
    return competitions[0]?.slug || "";
  }, [selectedSlug, initialCompetitionSlug, competitions]);

  const activeCompetition = useMemo(
    () => competitions.find((c) => c.slug === effectiveSlug) ?? null,
    [competitions, effectiveSlug]
  );

  const uniqueTeamCount = useMemo(() => {
    const teams = new Set(
      parsedRows.map((r) => r.team_name.trim().toLowerCase())
    );
    return teams.size;
  }, [parsedRows]);

  const missingUsers: MissingUserEntry[] = useMemo(
    () => pass1Result?.missing_users ?? [],
    [pass1Result]
  );

  const otherPass1Errors = useMemo(
    () =>
      (pass1Result?.skipped_errors ?? []).filter(
        (msg) => !msg.endsWith("account does not exist")
      ),
    [pass1Result]
  );

  const resetState = () => {
    setCsvText("");
    setParsedRows([]);
    setParseError(null);
    setPass1Result(null);
    setPass2Result(null);
    setSelectedSlug(initialCompetitionSlug || competitions[0]?.slug || "");
  };

  const handleClose = () => {
    if (isSubmitting) return;
    resetState();
    onClose();
  };

  const parseCsv = (text: string) => {
    const trimmed = text.replace(/^\uFEFF/, "").trim();
    if (!trimmed) {
      setParsedRows([]);
      setParseError(null);
      return;
    }

    try {
      const lines = trimmed
        .split(/\r?\n/)
        .map((l) => l.trim())
        .filter(Boolean);

      if (lines.length === 0) {
        setParsedRows([]);
        setParseError(null);
        return;
      }

      const firstRow = parseCsvLine(lines[0]);
      const isHeaderless = firstRow[0]?.includes("@");

      let startIdx = 0;
      let emailIdx = 0;
      let teamIdx = 1;
      let nameIdx = -1;
      let phoneIdx = -1;

      if (!isHeaderless) {
        if (lines.length < 2) {
          throw new Error(
            "Include a header row (email,team_name) and at least one data row."
          );
        }
        const header = firstRow.map((h) => h.toLowerCase());
        emailIdx = findColumn(header, ["email", "user_email"]);
        teamIdx = findColumn(header, ["team_name", "team"]);
        nameIdx = findColumn(header, ["name", "full_name"]);
        phoneIdx = findColumn(header, ["phone_number", "phone"]);

        if (emailIdx === -1 || teamIdx === -1) {
          throw new Error(
            "CSV header must include 'email' and 'team_name' (or 'team') columns."
          );
        }
        startIdx = 1;
      } else if (firstRow.length >= 3) {
        nameIdx = 2;
      }

      const rows: ParsedRow[] = [];
      for (let i = startIdx; i < lines.length; i++) {
        const cells = parseCsvLine(lines[i]);
        const email = (cells[emailIdx] || "").trim();
        const teamName = (cells[teamIdx] || "").trim();

        if (!email || !teamName) {
          throw new Error(
            `Row ${i + 1}: both email and team_name are required.`
          );
        }

        const name =
          nameIdx !== -1 && cells[nameIdx] ? cells[nameIdx].trim() : undefined;
        const phone =
          phoneIdx !== -1 && cells[phoneIdx]
            ? cells[phoneIdx].trim()
            : undefined;

        rows.push({
          email,
          team_name: teamName,
          name,
          phone_number: phone,
        });
      }

      setParsedRows(rows);
      setParseError(null);
    } catch (err: unknown) {
      setParseError(
        err instanceof Error ? err.message : "Failed to parse CSV"
      );
      setParsedRows([]);
    }
  };

  const handleFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = (ev) => {
      const content = String(ev.target?.result ?? "");
      setCsvText(content);
      parseCsv(content);
    };
    reader.readAsText(file);
  };

  const invalidateCaches = () => {
    queryClient.invalidateQueries({ queryKey: ["teams"] });
    queryClient.invalidateQueries({ queryKey: ["users"] });
  };

  const handleImportExisting = async () => {
    if (!activeCompetition || parsedRows.length === 0) return;
    setIsSubmitting(true);
    setParseError(null);
    try {
      const res = await eventsService.bulkAddEventUsers(eventSlug, {
        skip_missing_users: true,
        users: parsedRows.map((row) => ({
          email: row.email,
          name: row.name,
          phone_number: row.phone_number,
          role: "participant",
          competition_name: activeCompetition.slug,
          team_name: row.team_name,
        })),
      });
      setPass1Result(res);
      invalidateCaches();

      const missingCount = res.missing_users?.length ?? 0;
      if (missingCount === 0) {
        toast.success(
          `Assigned ${res.teams_assigned ?? 0} participant(s) in ${activeCompetition.name}.`,
          "Bulk import complete"
        );
      }
    } catch (err: unknown) {
      setParseError(apiErrorMessage(err, "Bulk import failed"));
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleCreateMissingAccounts = async () => {
    if (!activeCompetition || missingUsers.length === 0) return;
    setIsSubmitting(true);
    setParseError(null);
    try {
      const res = await eventsService.bulkAddEventUsers(eventSlug, {
        skip_missing_users: false,
        users: missingUsers.map((m) => ({
          email: m.email,
          name: m.name,
          phone_number: m.phone_number,
          role: "participant",
          competition_name: m.competition_name || activeCompetition.slug,
          team_name: m.team_name,
        })),
      });
      setPass2Result(res);
      invalidateCaches();
      toast.success(
        `Created ${res.total_created} account(s) and assigned ${res.teams_assigned ?? 0} to teams.`,
        "Accounts created"
      );
    } catch (err: unknown) {
      setParseError(
        apiErrorMessage(err, "Failed to create missing accounts")
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  const totalTeamsCreated =
    (pass1Result?.teams_created ?? 0) + (pass2Result?.teams_created ?? 0);
  const totalTeamsAssigned =
    (pass1Result?.teams_assigned ?? 0) + (pass2Result?.teams_assigned ?? 0);
  const combinedErrors = [
    ...otherPass1Errors,
    ...(pass2Result?.skipped_errors ?? []),
  ];

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      title="Bulk Import Users to Teams"
      description={
        activeCompetition
          ? `Assign participants to teams in "${activeCompetition.name}" by email and team name.`
          : "Assign participants to competition teams by email and team name."
      }
      footer={
        <div className="flex justify-end gap-2">
          {!pass1Result ? (
            <>
              <Button
                variant="outline"
                size="sm"
                type="button"
                onClick={handleClose}
                disabled={isSubmitting}
              >
                Cancel
              </Button>
              <Button
                variant="primary"
                size="sm"
                type="button"
                disabled={
                  !activeCompetition || parsedRows.length === 0 || isSubmitting
                }
                isLoading={isSubmitting}
                onClick={handleImportExisting}
              >
                Import {parsedRows.length > 0 ? `(${parsedRows.length})` : ""}
              </Button>
            </>
          ) : !pass2Result && missingUsers.length > 0 ? (
            <>
              <Button
                variant="outline"
                size="sm"
                type="button"
                onClick={handleClose}
                disabled={isSubmitting}
              >
                No, keep skipped
              </Button>
              <Button
                variant="primary"
                size="sm"
                type="button"
                isLoading={isSubmitting}
                onClick={handleCreateMissingAccounts}
                className="gap-1.5"
              >
                <UserPlus className="h-3.5 w-3.5" />
                <span>
                  Yes, create {missingUsers.length}{" "}
                  {missingUsers.length === 1 ? "account" : "accounts"} &amp;
                  assign
                </span>
              </Button>
            </>
          ) : (
            <Button
              variant="primary"
              size="sm"
              type="button"
              onClick={handleClose}
            >
              Done
            </Button>
          )}
        </div>
      }
    >
      <div className="space-y-4 font-sans text-xs">
        {/* Step 1: Input CSV */}
        {!pass1Result && (
          <div className="space-y-3">
            {competitions.length > 1 && (
              <Select
                label="Target competition"
                value={effectiveSlug}
                onChange={(e) => setSelectedSlug(e.target.value)}
                disabled={isSubmitting}
                className="text-xs h-9"
              >
                {competitions.map((comp) => (
                  <option key={comp.id} value={comp.slug}>
                    {comp.name} (/{comp.slug})
                  </option>
                ))}
              </Select>
            )}

            <div className="flex items-center justify-between gap-2">
              <span className="text-[11px] text-[var(--ink-secondary)] font-semibold uppercase">
                CSV Input (email, team_name)
              </span>
              <Button
                variant="ghost"
                size="sm"
                type="button"
                onClick={() => {
                  setCsvText(SAMPLE_CSV);
                  parseCsv(SAMPLE_CSV);
                }}
                className="h-6 text-[10px] px-2"
              >
                Insert sample
              </Button>
            </div>

            <p className="text-[11px] text-[var(--ink-secondary)]">
              Existing accounts are assigned to their teams right away (teams
              are created automatically if needed). Emails without an account
              will be skipped first, and you&apos;ll be asked whether to create
              them.
            </p>

            <textarea
              rows={6}
              value={csvText}
              onChange={(e) => {
                setCsvText(e.target.value);
                parseCsv(e.target.value);
              }}
              placeholder={SAMPLE_CSV}
              disabled={isSubmitting}
              className="w-full rounded-[14px] border border-[var(--card-line)] bg-[var(--card-solid)] p-3 font-mono text-xs text-[var(--ink-primary)] focus:border-[#83A2DB] focus:outline-none"
            />

            <div className="flex items-center justify-between border-t border-[var(--card-line)] pt-2">
              <label className="inline-flex items-center gap-1.5 text-xs cursor-pointer rounded-[12px] border border-[var(--card-line)] px-3 py-1.5 bg-[var(--card-solid-subtle)] hover:bg-[var(--color-blue-dim)] transition-colors">
                <Upload className="h-3.5 w-3.5 text-[var(--color-blue)]" />
                <span>Upload CSV</span>
                <input
                  type="file"
                  accept=".csv,text/csv"
                  onChange={handleFileUpload}
                  className="hidden"
                />
              </label>

              {parsedRows.length > 0 && (
                <div className="flex items-center gap-1 text-[var(--color-emerald)] font-semibold text-xs">
                  <CheckCircle2 className="h-4 w-4" />
                  <span>
                    {parsedRows.length}{" "}
                    {parsedRows.length === 1 ? "row" : "rows"} ·{" "}
                    {uniqueTeamCount}{" "}
                    {uniqueTeamCount === 1 ? "team" : "teams"}
                  </span>
                </div>
              )}
            </div>

            {parseError && (
              <div className="flex items-center gap-2 text-[var(--color-coral)] rounded-[14px] border border-[#CE6969]/30 p-3 bg-[var(--color-coral-dim)] text-xs font-semibold">
                <AlertCircle className="h-4 w-4 shrink-0" />
                <span>{parseError}</span>
              </div>
            )}
          </div>
        )}

        {/* Step 2: Pass 1 complete — ask about missing accounts if any */}
        {pass1Result && !pass2Result && (
          <div className="space-y-3">
            <Alert
              variant={missingUsers.length > 0 ? "warning" : "info"}
              title={
                missingUsers.length > 0
                  ? "Existing users imported — some accounts don't exist yet"
                  : "Bulk import complete"
              }
            >
              Processed {pass1Result.total_processed}{" "}
              {pass1Result.total_processed === 1 ? "row" : "rows"} for{" "}
              <strong>{activeCompetition?.name}</strong>.
            </Alert>

            <div className="rounded-[14px] border border-[var(--card-line)] divide-y divide-[var(--card-line)] bg-[var(--card-solid)] overflow-hidden">
              <div className="flex justify-between p-3">
                <span className="text-[var(--ink-secondary)]">
                  Assigned to teams
                </span>
                <strong className="font-mono text-[var(--ink-primary)]">
                  {pass1Result.teams_assigned ?? 0}
                </strong>
              </div>
              <div className="flex justify-between p-3">
                <span className="text-[var(--ink-secondary)]">
                  Teams created
                </span>
                <strong className="font-mono text-[var(--ink-primary)]">
                  {pass1Result.teams_created ?? 0}
                </strong>
              </div>
              <div className="flex justify-between p-3">
                <span className="text-[var(--ink-secondary)]">
                  Skipped (no account yet)
                </span>
                <strong className="font-mono text-[var(--color-coral)]">
                  {missingUsers.length}
                </strong>
              </div>
            </div>

            {missingUsers.length > 0 && (
              <div className="rounded-[14px] border border-[var(--color-blue-border)] bg-[var(--color-blue-dim)]/30 p-3.5 space-y-2.5">
                <div className="space-y-0.5">
                  <div className="font-semibold text-[var(--ink-primary)]">
                    Create {missingUsers.length} skipped{" "}
                    {missingUsers.length === 1 ? "account" : "accounts"} and add
                    them to teams?
                  </div>
                  <p className="text-[11px] text-[var(--ink-secondary)]">
                    These emails weren&apos;t found in the system. Confirm below
                    to create participant accounts for them and assign them to
                    their listed teams.
                  </p>
                </div>

                <div className="max-h-36 overflow-y-auto rounded-[10px] border border-[var(--card-line)] bg-[var(--card-solid)] divide-y divide-[var(--card-line)]">
                  {missingUsers.map((m, idx) => (
                    <div
                      key={`${m.email}-${m.team_name ?? ""}-${idx}`}
                      className="flex items-center justify-between gap-2 px-3 py-1.5 text-[11px]"
                    >
                      <span className="font-mono text-[var(--ink-primary)] truncate">
                        {m.email}
                      </span>
                      {m.team_name && (
                        <Badge variant="subtle" size="sm">
                          {m.team_name}
                        </Badge>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            )}

            {otherPass1Errors.length > 0 && (
              <div className="rounded-[14px] border border-[var(--card-line)] p-3 bg-[var(--card-solid-subtle)] max-h-32 overflow-y-auto space-y-1">
                <span className="font-bold text-[11px] uppercase text-[var(--ink-primary)]">
                  Other warnings:
                </span>
                {otherPass1Errors.map((msg, i) => (
                  <div
                    key={i}
                    className="text-[11px] text-[var(--ink-secondary)] font-mono"
                  >
                    • {msg}
                  </div>
                ))}
              </div>
            )}

            {parseError && (
              <div className="flex items-center gap-2 text-[var(--color-coral)] rounded-[14px] border border-[#CE6969]/30 p-3 bg-[var(--color-coral-dim)] text-xs font-semibold">
                <AlertCircle className="h-4 w-4 shrink-0" />
                <span>{parseError}</span>
              </div>
            )}
          </div>
        )}

        {/* Step 3: Final summary after creating missing accounts */}
        {pass1Result && pass2Result && (
          <div className="space-y-3">
            <Alert variant="info" title="All accounts created and assigned">
              Finished importing into{" "}
              <strong>{activeCompetition?.name}</strong>.
            </Alert>

            <div className="rounded-[14px] border border-[var(--card-line)] divide-y divide-[var(--card-line)] bg-[var(--card-solid)] overflow-hidden">
              <div className="flex justify-between p-3">
                <span className="text-[var(--ink-secondary)]">
                  New accounts created
                </span>
                <strong className="font-mono text-[var(--color-emerald)]">
                  {pass2Result.total_created}
                </strong>
              </div>
              <div className="flex justify-between p-3">
                <span className="text-[var(--ink-secondary)]">
                  Total team assignments
                </span>
                <strong className="font-mono text-[var(--ink-primary)]">
                  {totalTeamsAssigned}
                </strong>
              </div>
              <div className="flex justify-between p-3">
                <span className="text-[var(--ink-secondary)]">
                  Total teams created
                </span>
                <strong className="font-mono text-[var(--ink-primary)]">
                  {totalTeamsCreated}
                </strong>
              </div>
            </div>

            {combinedErrors.length > 0 && (
              <div className="rounded-[14px] border border-[var(--card-line)] p-3 bg-[var(--card-solid-subtle)] max-h-32 overflow-y-auto space-y-1">
                <span className="font-bold text-[11px] uppercase text-[var(--ink-primary)]">
                  Warnings:
                </span>
                {combinedErrors.map((msg, i) => (
                  <div
                    key={i}
                    className="text-[11px] text-[var(--ink-secondary)] font-mono"
                  >
                    • {msg}
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
      </div>
    </Dialog>
  );
}
