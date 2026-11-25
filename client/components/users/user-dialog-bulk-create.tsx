"use client";

import React, { useMemo, useState } from "react";
import {
  BulkCreateResponse,
  BulkCreateUsersRequest,
  CreateUserRequest,
} from "@/types/users";
import { UserRole } from "@/types/auth";
import type { Competition } from "@/types/competitions";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Alert } from "@/components/ui/alert";
import { Upload, CheckCircle2, AlertCircle } from "lucide-react";

interface UserBulkCreateDialogProps {
  open: boolean;
  onClose: () => void;
  onSubmit: (data: BulkCreateUsersRequest) => Promise<BulkCreateResponse>;
  competitions?: ReadonlyArray<Competition>;
  isLoading: boolean;
}

function escapeCsvCell(val: string): string {
  return /[",\r\n]/.test(val) ? `"${val.replace(/"/g, '""')}"` : val;
}

function buildSampleCsv(competitions: ReadonlyArray<Competition>): string {
  if (competitions.length >= 2) {
    const c1 = escapeCsvCell(competitions[0].name);
    const c2 = escapeCsvCell(competitions[1].name);
    return [
      "email,name,role,phone_number,competition_name,team_name",
      `alice@example.com,Alice Smith,participant,+15550101,${c1},Neural Ninjas`,
      `bob@example.com,Bob Jones,participant,+15550102,${c1},Neural Ninjas`,
      `alice@example.com,Alice Smith,participant,+15550101,${c2},Pixel Forge`,
      "charlie@example.com,Charlie Brown,judge,,,",
    ].join("\n");
  }
  if (competitions.length === 1) {
    const c1 = escapeCsvCell(competitions[0].name);
    return [
      "email,name,role,phone_number,competition_name,team_name",
      `alice@example.com,Alice Smith,participant,+15550101,${c1},Neural Ninjas`,
      `bob@example.com,Bob Jones,participant,+15550102,${c1},Neural Ninjas`,
      "charlie@example.com,Charlie Brown,judge,,,",
    ].join("\n");
  }
  return [
    "email,name,role,phone_number",
    "alice@example.com,Alice Smith,participant,+15550101",
    "bob@example.com,Bob Jones,participant,+15550102",
    "charlie@example.com,Charlie Brown,judge,",
  ].join("\n");
}

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

export function UserBulkCreateDialog({
  open,
  onClose,
  onSubmit,
  competitions = [],
  isLoading,
}: UserBulkCreateDialogProps) {
  const [csvText, setCsvText] = useState("");
  const [parsedUsers, setParsedUsers] = useState<CreateUserRequest[]>([]);
  const [parseError, setParseError] = useState<string | null>(null);
  const [result, setResult] = useState<BulkCreateResponse | null>(null);

  const sampleCsv = useMemo(() => buildSampleCsv(competitions), [competitions]);
  const hasTeamColumns = useMemo(
    () => parsedUsers.some((u) => Boolean(u.team_name)),
    [parsedUsers]
  );

  const uniqueUserCount = useMemo(() => {
    const emails = new Set(parsedUsers.map((u) => u.email.trim().toLowerCase()));
    return emails.size;
  }, [parsedUsers]);

  const resetDialog = () => {
    setCsvText("");
    setParsedUsers([]);
    setParseError(null);
    setResult(null);
  };

  const handleClose = () => {
    resetDialog();
    onClose();
  };

  const parseCsv = (text: string) => {
    try {
      const lines = text
        .replace(/^\uFEFF/, "")
        .trim()
        .split(/\r?\n/)
        .map((l) => l.trim())
        .filter(Boolean);

      if (lines.length < 2) {
        throw new Error("CSV must include a header line and at least one user record.");
      }

      const header = parseCsvLine(lines[0]).map((h) => h.toLowerCase());
      const emailIdx = findColumn(header, ["email"]);
      const nameIdx = findColumn(header, ["name"]);

      if (emailIdx === -1 || nameIdx === -1) {
        throw new Error("CSV header must include 'email' and 'name' columns.");
      }

      const roleIdx = findColumn(header, ["role"]);
      const phoneIdx = findColumn(header, ["phone_number", "phone"]);
      const compIdx = findColumn(header, ["competition_name", "competition", "competition_slug"]);
      const teamIdx = findColumn(header, ["team_name", "team"]);

      const users: CreateUserRequest[] = [];

      for (let i = 1; i < lines.length; i++) {
        const row = parseCsvLine(lines[i]);
        const email = row[emailIdx] || "";
        const name = row[nameIdx] || "";

        if (!email || !name) {
          throw new Error(`Row ${i + 1}: email and name are required.`);
        }

        const role = (roleIdx !== -1 && row[roleIdx] ? row[roleIdx].toLowerCase() : "participant") as UserRole;
        const phone = phoneIdx !== -1 && row[phoneIdx] ? row[phoneIdx] : undefined;
        const competitionName = compIdx !== -1 && row[compIdx] ? row[compIdx] : undefined;
        const teamName = teamIdx !== -1 && row[teamIdx] ? row[teamIdx] : undefined;

        if (teamName && !competitionName) {
          throw new Error(`Row ${i + 1}: competition_name is required when team_name is provided.`);
        }

        users.push({
          email,
          name,
          role,
          phone_number: phone,
          competition_name: competitionName,
          team_name: teamName,
        });
      }

      setParsedUsers(users);
      setParseError(null);
    } catch (err: unknown) {
      setParseError(err instanceof Error ? err.message : "Failed to parse CSV");
      setParsedUsers([]);
    }
  };

  const handleFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    const reader = new FileReader();
    reader.onload = (event) => {
      const content = event.target?.result as string;
      setCsvText(content);
      parseCsv(content);
    };
    reader.readAsText(file);
  };

  const handleSubmit = async () => {
    if (parsedUsers.length === 0) return;
    try {
      const res = await onSubmit({ users: parsedUsers });
      setResult(res);
    } catch (err: unknown) {
      setParseError(err instanceof Error ? err.message : "Import failed");
    }
  };

  const hasDuplicates = parsedUsers.length > uniqueUserCount;

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      title="Bulk Import Users (CSV)"
      description="Import users and optionally assign participants to competitions and teams. Duplicate emails across competitions are automatically deduplicated."
      footer={
        <div className="flex justify-end gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={handleClose}
            disabled={isLoading}
          >
            {result ? "Close" : "Cancel"}
          </Button>

          {!result && (
            <Button
              variant="primary"
              size="sm"
              disabled={parsedUsers.length === 0 || isLoading}
              onClick={handleSubmit}
              isLoading={isLoading}
            >
              {hasDuplicates
                ? `Import ${uniqueUserCount} Users (${parsedUsers.length} rows)`
                : `Import ${parsedUsers.length} Users`}
            </Button>
          )}
        </div>
      }
    >
      <div className="space-y-4 font-mono text-xs">
        {result ? (
          <div className="space-y-3">
            <Alert variant="info" title="Bulk import complete">
              Processed {result.total_processed} row{result.total_processed === 1 ? "" : "s"}.
            </Alert>

            <div className="rounded-[16px] border border-[var(--card-line)] divide-y divide-[var(--card-line)] bg-[var(--card-solid)] overflow-hidden">
              <div className="flex justify-between p-3">
                <span className="text-[var(--ink-secondary)]">Accounts Created / Enrolled</span>
                <strong className="text-[var(--ink-primary)] font-mono">{result.total_created}</strong>
              </div>
              {(hasTeamColumns || (result.teams_created ?? 0) > 0) && (
                <div className="flex justify-between p-3">
                  <span className="text-[var(--ink-secondary)]">Teams Created</span>
                  <strong className="text-[var(--ink-primary)] font-mono">{result.teams_created ?? 0}</strong>
                </div>
              )}
              {(hasTeamColumns || (result.teams_assigned ?? 0) > 0) && (
                <div className="flex justify-between p-3">
                  <span className="text-[var(--ink-secondary)]">Team Assignments</span>
                  <strong className="text-[var(--ink-primary)] font-mono">{result.teams_assigned ?? 0}</strong>
                </div>
              )}
              <div className="flex justify-between p-3">
                <span className="text-[var(--ink-secondary)]">Skipped Records</span>
                <span className="font-semibold text-[var(--ink-primary)] font-mono">
                  {result.skipped_count}
                </span>
              </div>
            </div>

            {result.skipped_errors && result.skipped_errors.length > 0 && (
              <div className="rounded-[14px] border border-[var(--card-line)] p-3 bg-[var(--card-solid-subtle)] max-h-36 overflow-y-auto space-y-1">
                <span className="font-bold text-[11px] uppercase text-[var(--ink-primary)]">Skipped Warnings:</span>
                {result.skipped_errors.map((msg, i) => (
                  <div key={i} className="text-[10px] text-[var(--ink-secondary)] font-mono">
                    • {msg}
                  </div>
                ))}
              </div>
            )}
          </div>
        ) : (
          <div className="space-y-3">
            <div className="flex items-center justify-between gap-2">
              <span className="text-[11px] text-[var(--ink-secondary)] font-semibold uppercase">
                CSV Input Data
              </span>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => {
                  setCsvText(sampleCsv);
                  parseCsv(sampleCsv);
                }}
                className="h-6 text-[10px] px-2"
              >
                Insert Sample Data
              </Button>
            </div>

            <p className="text-[11px] font-sans text-[var(--ink-secondary)]">
              Required: <code className="font-mono text-[var(--ink-primary)]">email</code>,{" "}
              <code className="font-mono text-[var(--ink-primary)]">name</code>. Optional:{" "}
              <code className="font-mono text-[var(--ink-primary)]">role</code>,{" "}
              <code className="font-mono text-[var(--ink-primary)]">phone_number</code>,{" "}
              <code className="font-mono text-[var(--ink-primary)]">competition_name</code>,{" "}
              <code className="font-mono text-[var(--ink-primary)]">team_name</code>.
            </p>

            {competitions.length > 0 && (
              <p className="text-[11px] font-sans text-[var(--ink-tertiary)]">
                Available competitions:{" "}
                <span className="font-mono text-[var(--ink-secondary)]">
                  {competitions.map((c) => c.name).join(", ")}
                </span>
              </p>
            )}

            <textarea
              rows={6}
              value={csvText}
              onChange={(e) => {
                setCsvText(e.target.value);
                parseCsv(e.target.value);
              }}
              placeholder={sampleCsv}
              className="w-full rounded-[14px] border border-[var(--card-line)] bg-[var(--card-solid)] p-3 font-mono text-xs text-[var(--ink-primary)] focus:border-[#83A2DB] focus:outline-none"
            />

            <div className="flex items-center justify-between border-t border-[var(--card-line)] pt-2">
              <label className="inline-flex items-center gap-1.5 text-xs cursor-pointer rounded-[12px] border border-[var(--card-line)] px-3.5 py-1.5 bg-[var(--card-solid-subtle)] hover:bg-[var(--color-blue-dim)] transition-colors">
                <Upload className="h-3.5 w-3.5 text-[var(--color-blue)]" />
                <span>Upload CSV File</span>
                <input
                  type="file"
                  accept=".csv,text/csv"
                  onChange={handleFileUpload}
                  className="hidden"
                />
              </label>

              {parsedUsers.length > 0 && (
                <div className="flex items-center gap-1 text-[#2EB88A] font-semibold text-xs">
                  <CheckCircle2 className="h-4 w-4" />
                  <span>
                    {hasDuplicates
                      ? `${parsedUsers.length} rows · ${uniqueUserCount} unique users`
                      : `${parsedUsers.length} valid rows detected`}
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
      </div>
    </Dialog>
  );
}
