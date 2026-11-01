"use client";

import React, { useEffect, useMemo, useState } from "react";
import { ImportTeamCriteriaRequest, TeamCriterionRow } from "@/types/judging";
import { useTeams } from "@/hooks/use-teams";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Upload, Download } from "lucide-react";

interface ParsedRow {
  line: number;
  team: string;
  criterion: string;
  max: number;
}

// Minimal RFC-4180 style parser: quoted fields, escaped quotes, CRLF, BOM.
function parseCsv(text: string): string[][] {
  const rows: string[][] = [];
  let row: string[] = [];
  let field = "";
  let quoted = false;
  const src = text.replace(/^﻿/, "");
  for (let i = 0; i < src.length; i++) {
    const ch = src[i];
    if (quoted) {
      if (ch === '"') {
        if (src[i + 1] === '"') {
          field += '"';
          i++;
        } else quoted = false;
      } else field += ch;
    } else if (ch === '"') quoted = true;
    else if (ch === ",") {
      row.push(field);
      field = "";
    } else if (ch === "\n" || ch === "\r") {
      if (ch === "\r" && src[i + 1] === "\n") i++;
      row.push(field);
      field = "";
      if (row.some((c) => c.trim() !== "")) rows.push(row);
      row = [];
    } else field += ch;
  }
  row.push(field);
  if (row.some((c) => c.trim() !== "")) rows.push(row);
  return rows;
}

const norm = (s: string) => s.trim().toLowerCase().replace(/\s+/g, " ");

const TEMPLATE = "team,criterion_name,max_score\nTeam Alpha,Problem Statement Fit,20\nTeam Beta,Hardware Demo,30\n";


export type TeamCriteriaPickerState =
  | { status: "empty"; payload: null }
  | { status: "invalid"; payload: null; message: string }
  | { status: "valid"; payload: ImportTeamCriteriaRequest };

interface PickerProps {
  onChange: (state: TeamCriteriaPickerState) => void;
}

/** CSV chooser + weight + match preview. Reports the request payload once everything is valid. */
export function TeamCriteriaPicker({ onChange }: PickerProps) {
  const { data: teamsData } = useTeams({ limit: 100 });
  const [weight, setWeight] = useState("");
  const [rows, setRows] = useState<ParsedRow[]>([]);
  const [fileError, setFileError] = useState<string | null>(null);

  const teamByName = useMemo(() => {
    const m = new Map<string, string>();
    (teamsData?.data ?? []).forEach((t) => m.set(norm(t.name), t.id));
    return m;
  }, [teamsData]);

  const analysed = useMemo(() => {
    const seen = new Set<string>();
    return rows.map((r) => {
      const key = norm(r.team);
      const teamId = teamByName.get(key);
      let problem: string | null = null;
      if (!teamId) problem = "No team with this name";
      else if (seen.has(key)) problem = "Duplicate team";
      else if (!r.criterion.trim()) problem = "Missing criterion name";
      else if (r.criterion.trim().length > 150) problem = "Criterion name too long";
      else if (!Number.isInteger(r.max) || r.max < 1 || r.max > 999) problem = "Max score must be a whole number 1-999";
      seen.add(key);
      return { ...r, teamId, problem };
    });
  }, [rows, teamByName]);

  const weightNum = Number(weight);
  const weightOk = weight.trim() !== "" && Number.isFinite(weightNum) && weightNum > 0;

  useEffect(() => {
    if (rows.length === 0 && !fileError) {
      onChange({ status: "empty", payload: null });
      return;
    }
    if (fileError) {
      onChange({ status: "invalid", payload: null, message: fileError });
      return;
    }
    const bad = analysed.find((r) => r.problem);
    if (bad) {
      onChange({ status: "invalid", payload: null, message: `Team CSV line ${bad.line}: ${bad.problem}.` });
      return;
    }
    if (!weightOk) {
      onChange({ status: "invalid", payload: null, message: "Enter a weight for the team criterion." });
      return;
    }
    const payloadRows: TeamCriterionRow[] = analysed.map((r) => ({
      team_id: r.teamId as string,
      criterion_name: r.criterion,
      max_score: r.max,
    }));
    onChange({ status: "valid", payload: { weight: weightNum, rows: payloadRows } });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [analysed, weightOk, weightNum, fileError, rows.length]);

  const handleFile = async (file: File | undefined) => {
    setFileError(null);
    setRows([]);
    if (!file) return;
    const parsed = parseCsv(await file.text());
    if (parsed.length < 2) {
      setFileError("The file needs a header row and at least one team.");
      return;
    }
    const header = parsed[0].map(norm);
    const col = (names: string[]) => header.findIndex((h) => names.includes(h));
    const ti = col(["team", "team name", "team_name"]);
    const ci = col(["criterion_name", "criterion name", "criterion", "criteria"]);
    const mi = col(["max_score", "max score", "max", "value"]);
    if (ti < 0 || ci < 0 || mi < 0) {
      setFileError("Header must contain: team, criterion_name, max_score.");
      return;
    }
    setRows(
      parsed.slice(1).map((cells, i) => ({
        line: i + 2,
        team: (cells[ti] ?? "").trim(),
        criterion: (cells[ci] ?? "").trim(),
        max: Number((cells[mi] ?? "").trim()),
      }))
    );
  };

  const downloadTemplate = () => {
    const url = URL.createObjectURL(new Blob([TEMPLATE], { type: "text/csv" }));
    const a = document.createElement("a");
    a.href = url;
    a.download = "team-criteria-template.csv";
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="space-y-4 font-sans text-xs">
      <div className="flex flex-wrap items-center gap-2">
        <label className="inline-flex items-center gap-1.5 cursor-pointer rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid)] px-3 py-2 font-semibold text-[var(--ink-primary)] hover:bg-[var(--card-solid-subtle)]">
          <Upload className="h-3.5 w-3.5" />
          <span>Choose CSV</span>
          <input
            type="file"
            accept=".csv,text/csv"
            className="sr-only"
            onChange={(e) => {
              handleFile(e.target.files?.[0]);
              e.target.value = "";
            }}
          />
        </label>
        <Button type="button" variant="ghost" size="sm" onClick={downloadTemplate} className="gap-1.5 text-xs">
          <Download className="h-3.5 w-3.5" />
          Template
        </Button>
        <span className="text-[var(--ink-tertiary)]">Columns: team, criterion_name, max_score</span>
      </div>

      <div className="max-w-[220px]">
        <Input
          label="Weight (same for every team)"
          type="number"
          min={0}
          step="any"
          value={weight}
          onChange={(e) => setWeight(e.target.value)}
          placeholder="e.g. 20"
        />
      </div>

      {fileError && <p className="text-[var(--color-coral)]">{fileError}</p>}

      {analysed.length > 0 && (
        <div className="rounded-[14px] border border-[var(--card-line)] divide-y divide-[var(--card-line)] bg-[var(--card-solid)] max-h-[260px] overflow-y-auto">
          {analysed.map((r) => (
            <div key={r.line} className="flex items-center justify-between gap-3 px-3.5 py-2">
              <div className="min-w-0">
                <div className="font-semibold text-[var(--ink-primary)] truncate">{r.team || "(blank)"}</div>
                <div className="text-[var(--ink-secondary)] truncate">{r.criterion || "(no criterion)"}</div>
              </div>
              <div className="shrink-0 flex items-center gap-2">
                <span className="tabular-nums text-[var(--ink-secondary)]">max {Number.isNaN(r.max) ? "?" : r.max}</span>
                {r.problem ? (
                  <Badge variant="subtle" size="sm">
                    <span className="text-[var(--color-coral)]">Line {r.line}: {r.problem}</span>
                  </Badge>
                ) : (
                  <Badge variant="subtle" size="sm">Matched</Badge>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
      {analysed.some((r) => r.problem) && (
        <p className="text-[var(--ink-secondary)]">Fix the highlighted rows in the CSV and choose it again.</p>
      )}
    </div>
  );
}
