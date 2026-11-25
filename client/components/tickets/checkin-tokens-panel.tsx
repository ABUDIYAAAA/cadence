"use client";

import React, { useEffect, useMemo, useRef, useState } from "react";
import {
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Clock,
  Copy,
  Download,
  MoreHorizontal,
  Pencil,
  Plus,
  RefreshCw,
  Search,
  UserCheck,
  UserX,
  Users,
  X,
} from "lucide-react";
import {
  useClearSessionCode,
  useCreateScanSession,
  useGenerateSessionCode,
  useScanSessions,
  useSessionReport,
  useSetScanSessionOpen,
} from "@/hooks/use-tickets";
import { useTenant } from "@/providers/tenant-provider";
import { useToast } from "@/providers/toast-provider";
import {
  buildCreateScanSessionPayload,
  normalizeCheckInCode,
  validateCustomCheckInCode,
} from "@/lib/tickets-utils";
import { apiErrorMessage, cn } from "@/lib/utils";
import { downloadCsv, slugify, toCsv } from "@/lib/csv";
import type { ScanKind, ScanSession, SessionPerson } from "@/types/tickets";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { SkeletonCard } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

const card =
  "bg-[var(--card-solid)] border border-[var(--card-line)] rounded-[16px] p-4 sm:p-5 space-y-4";

type RosterFilter = "all" | "present" | "absent";
type RosterSort = "latest" | "earliest" | "name" | "team";

function formatTimestamp(
  iso?: string | null
): { date: string; time: string; full: string } | null {
  if (!iso) return null;
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return null;
  const date = d.toLocaleDateString([], { month: "short", day: "numeric" });
  const time = d.toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
  return { date, time, full: `${date}, ${time}` };
}

function formatCheckedInBy(p: SessionPerson): string {
  if (!p.present) return "—";
  if (p.scanned_by_id && p.scanned_by_id === p.user_id) {
    return "Self (token)";
  }
  if (p.scanned_by_name) {
    return p.scanned_by_name;
  }
  return "Self / Staff";
}

function CollapsibleSection({
  title,
  summary,
  open,
  onToggle,
  children,
}: {
  title: string;
  summary?: string;
  open: boolean;
  onToggle: () => void;
  children: React.ReactNode;
}) {
  return (
    <div className="rounded-[12px] border border-[var(--card-line)] overflow-hidden">
      <button
        type="button"
        onClick={onToggle}
        aria-expanded={open}
        className="w-full flex items-center justify-between gap-3 px-3.5 py-2.5 text-left bg-[var(--card-solid-subtle)]/50 hover:bg-[var(--card-solid-subtle)] transition-colors cursor-pointer"
      >
        <div className="flex items-center gap-2 min-w-0">
          {open ? (
            <ChevronDown className="h-4 w-4 text-[var(--ink-tertiary)] shrink-0" />
          ) : (
            <ChevronRight className="h-4 w-4 text-[var(--ink-tertiary)] shrink-0" />
          )}
          <span className="text-xs font-semibold text-[var(--ink-primary)]">
            {title}
          </span>
        </div>
        {summary && (
          <span className="text-[11px] text-[var(--ink-secondary)] truncate tabular-nums">
            {summary}
          </span>
        )}
      </button>
      {open && (
        <div className="p-3.5 border-t border-[var(--card-line)] space-y-3 bg-[var(--card-solid)]">
          {children}
        </div>
      )}
    </div>
  );
}

function ExportMenu({
  totalCount,
  presentCount,
  absentCount,
  onExport,
}: {
  totalCount: number;
  presentCount: number;
  absentCount: number;
  onExport: (scope: RosterFilter) => void;
}) {
  const [open, setOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!open) return;
    const handleClickOutside = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", handleClickOutside);
    document.addEventListener("keydown", handleEscape);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
      document.removeEventListener("keydown", handleEscape);
    };
  }, [open]);

  return (
    <div ref={menuRef} className="relative">
      <Button
        variant="outline"
        size="sm"
        type="button"
        disabled={totalCount === 0}
        onClick={() => setOpen((prev) => !prev)}
        className="h-8 px-2.5 text-xs gap-1.5"
      >
        <Download className="h-3.5 w-3.5" />
        <span>Export CSV</span>
        <ChevronDown className="h-3 w-3 opacity-70" />
      </Button>

      {open && (
        <div className="absolute right-0 mt-1.5 w-52 rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid)] shadow-lg p-1 z-20 text-xs">
          <button
            type="button"
            disabled={totalCount === 0}
            onClick={() => {
              onExport("all");
              setOpen(false);
            }}
            className="w-full flex items-center justify-between px-2.5 py-1.5 rounded-[8px] text-[var(--ink-primary)] hover:bg-[var(--card-solid-subtle)] disabled:opacity-40 cursor-pointer"
          >
            <span>All attendees</span>
            <span className="font-mono text-[11px] text-[var(--ink-tertiary)]">
              {totalCount}
            </span>
          </button>
          <button
            type="button"
            disabled={presentCount === 0}
            onClick={() => {
              onExport("present");
              setOpen(false);
            }}
            className="w-full flex items-center justify-between px-2.5 py-1.5 rounded-[8px] text-[var(--ink-primary)] hover:bg-[var(--card-solid-subtle)] disabled:opacity-40 cursor-pointer"
          >
            <span>Checked in only</span>
            <span className="font-mono text-[11px] text-[var(--color-emerald)]">
              {presentCount}
            </span>
          </button>
          <button
            type="button"
            disabled={absentCount === 0}
            onClick={() => {
              onExport("absent");
              setOpen(false);
            }}
            className="w-full flex items-center justify-between px-2.5 py-1.5 rounded-[8px] text-[var(--ink-primary)] hover:bg-[var(--card-solid-subtle)] disabled:opacity-40 cursor-pointer"
          >
            <span>Not checked in</span>
            <span className="font-mono text-[11px] text-[var(--color-coral)]">
              {absentCount}
            </span>
          </button>
        </div>
      )}
    </div>
  );
}

function SessionDetailedStats({
  session,
  onCopyToken,
}: {
  session: ScanSession;
  onCopyToken: (code: string) => void;
}) {
  const { data, isLoading, isFetching, refetch } = useSessionReport(session.id);

  const [showBreakdown, setShowBreakdown] = useState(false);
  const [showRoster, setShowRoster] = useState(false);

  const [view, setView] = useState<RosterFilter>("all");
  const [search, setSearch] = useState("");
  const [teamFilter, setTeamFilter] = useState("");
  const [sortBy, setSortBy] = useState<RosterSort>("latest");

  const people = useMemo(() => data?.people ?? [], [data]);

  const uniqueTeams = useMemo(() => {
    const names = new Set<string>();
    for (const p of people) {
      if (p.team_name) names.add(p.team_name);
    }
    return Array.from(names).sort((a, b) => a.localeCompare(b));
  }, [people]);

  const stats = useMemo(() => {
    const expected = data?.eligible ?? people.length;
    const presentList = people.filter((p) => p.present);
    const absentList = people.filter((p) => !p.present);
    const presentCount = data?.scanned ?? presentList.length;
    const absentCount = absentList.length;
    const rate = expected > 0 ? Math.round((presentCount / expected) * 100) : 0;

    const times = presentList
      .map((p) => (p.scanned_at ? new Date(p.scanned_at).getTime() : NaN))
      .filter((t) => !Number.isNaN(t))
      .sort((a, b) => a - b);

    const firstAt = times.length > 0 ? new Date(times[0]).toISOString() : null;
    const latestAt =
      times.length > 0 ? new Date(times[times.length - 1]).toISOString() : null;

    const selfCount = presentList.filter(
      (p) => p.scanned_by_id && p.scanned_by_id === p.user_id
    ).length;
    const staffCount = presentCount - selfCount;

    const teamMap = new Map<string, { total: number; present: number }>();
    for (const p of people) {
      if (!p.team_name) continue;
      const entry = teamMap.get(p.team_name) ?? { total: 0, present: 0 };
      entry.total++;
      if (p.present) entry.present++;
      teamMap.set(p.team_name, entry);
    }
    let completeTeams = 0;
    for (const entry of teamMap.values()) {
      if (entry.total > 0 && entry.present === entry.total) completeTeams++;
    }

    return {
      expected,
      presentCount,
      absentCount,
      rate,
      firstAt,
      latestAt,
      selfCount,
      staffCount,
      totalTeams: teamMap.size,
      completeTeams,
    };
  }, [data, people]);

  const filteredPeople = useMemo(() => {
    const q = search.trim().toLowerCase();
    const list = people.filter((p) => {
      if (view === "present" && !p.present) return false;
      if (view === "absent" && p.present) return false;
      if (teamFilter === "__none__" && p.team_name) return false;
      if (teamFilter && teamFilter !== "__none__" && p.team_name !== teamFilter) {
        return false;
      }
      if (!q) return true;
      return (
        p.name.toLowerCase().includes(q) ||
        p.email.toLowerCase().includes(q) ||
        (p.team_name && p.team_name.toLowerCase().includes(q)) ||
        (p.phone_number && p.phone_number.toLowerCase().includes(q)) ||
        (p.scanned_by_name && p.scanned_by_name.toLowerCase().includes(q))
      );
    });

    const sorted = [...list];
    sorted.sort((a, b) => {
      if (sortBy === "latest" || sortBy === "earliest") {
        const ta = a.scanned_at ? new Date(a.scanned_at).getTime() : -1;
        const tb = b.scanned_at ? new Date(b.scanned_at).getTime() : -1;
        if (ta !== tb) {
          if (ta === -1) return 1;
          if (tb === -1) return -1;
          return sortBy === "latest" ? tb - ta : ta - tb;
        }
        return a.name.localeCompare(b.name);
      }
      if (sortBy === "team") {
        const teamCmp = (a.team_name || "zzz").localeCompare(
          b.team_name || "zzz"
        );
        if (teamCmp !== 0) return teamCmp;
        return a.name.localeCompare(b.name);
      }
      return a.name.localeCompare(b.name);
    });
    return sorted;
  }, [people, view, teamFilter, search, sortBy]);

  const exportCsv = (scope: RosterFilter) => {
    const rows =
      scope === "all"
        ? people
        : people.filter((p) => (scope === "present" ? p.present : !p.present));
    const csv = toCsv(
      [
        "Session",
        "Token",
        "Name",
        "Email",
        "Phone",
        "Team",
        "Team Role",
        "Attendee Type",
        "Status",
        "Checked In At (ISO)",
        "Checked In At (Local)",
        "Checked In By",
      ],
      rows.map((p) => {
        const fmt = formatTimestamp(p.scanned_at);
        return [
          session.name,
          session.checkin_code || "",
          p.name,
          p.email,
          p.phone_number || "",
          p.team_name || "",
          p.team_name ? (p.is_leader ? "Leader" : "Member") : "",
          p.is_external ? "External" : "Internal",
          p.present ? "Checked In" : "Not Checked In",
          p.scanned_at || "",
          fmt ? fmt.full : "",
          formatCheckedInBy(p),
        ];
      })
    );
    const suffix =
      scope === "all"
        ? "full-report"
        : scope === "present"
        ? "checked-in"
        : "not-checked-in";
    downloadCsv(`${slugify(session.name)}-${suffix}.csv`, csv);
  };

  if (isLoading || !data) {
    return (
      <div className={card}>
        <SkeletonCard />
      </div>
    );
  }

  const firstFmt = formatTimestamp(stats.firstAt);
  const latestFmt = formatTimestamp(stats.latestAt);

  const breakdownSummary = [
    `${stats.selfCount} self · ${stats.staffCount} staff`,
    latestFmt ? `Latest ${latestFmt.time}` : null,
    stats.totalTeams > 0
      ? `${stats.completeTeams}/${stats.totalTeams} teams`
      : null,
  ]
    .filter(Boolean)
    .join(" · ");

  return (
    <div className={card}>
      {/* Compact Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div className="flex flex-wrap items-center gap-2 min-w-0">
          <h3 className="text-base font-bold tracking-tight text-[var(--ink-primary)] truncate">
            {session.name}
          </h3>
          {session.checkin_code && (
            <button
              type="button"
              onClick={() => onCopyToken(session.checkin_code!)}
              title="Copy check-in token"
              className="inline-flex items-center gap-1 font-mono text-xs font-semibold tracking-wider px-2 py-0.5 rounded-[6px] border border-[var(--color-blue-border)] bg-[var(--color-blue-dim)] text-[var(--color-blue)] hover:opacity-90 cursor-pointer"
            >
              <span>{session.checkin_code}</span>
              <Copy className="h-3 w-3" />
            </button>
          )}
          <Badge variant={session.is_open ? "emerald" : "subtle"} size="sm" dot>
            {session.is_open ? "Open" : "Closed"}
          </Badge>
          {session.competition_name && (
            <Badge variant="subtle" size="sm">
              {session.competition_name}
            </Badge>
          )}
        </div>

        <div className="flex items-center gap-2 shrink-0">
          <Button
            variant="outline"
            size="sm"
            onClick={() => refetch()}
            disabled={isFetching}
            className="h-8 px-2.5 text-xs gap-1.5"
            title="Refresh attendance stats"
          >
            <RefreshCw
              className={`h-3.5 w-3.5 ${isFetching ? "animate-spin" : ""}`}
            />
            <span className="hidden sm:inline">Refresh</span>
          </Button>
          <ExportMenu
            totalCount={people.length}
            presentCount={stats.presentCount}
            absentCount={stats.absentCount}
            onExport={exportCsv}
          />
        </div>
      </div>

      {/* Essential Progress Strip */}
      <div className="space-y-2">
        <div className="flex flex-wrap items-baseline justify-between gap-2 text-xs">
          <div className="flex items-baseline gap-2">
            <span className="text-lg font-bold tabular-nums text-[var(--ink-primary)]">
              {stats.presentCount}
            </span>
            <span className="text-[var(--ink-secondary)] tabular-nums">
              of {stats.expected} checked in ({stats.rate}%)
            </span>
          </div>
          <span className="text-[var(--color-coral)] font-medium tabular-nums">
            {stats.absentCount} pending
          </span>
        </div>
        <div className="h-2 w-full overflow-hidden rounded-full bg-[var(--card-solid-subtle)]">
          <div
            className="h-full rounded-full bg-[var(--color-emerald)] transition-[width] duration-300"
            style={{ width: `${Math.min(stats.rate, 100)}%` }}
          />
        </div>
      </div>

      {/* Collapsible: Breakdown & Timing */}
      <CollapsibleSection
        title="Stats & timing"
        summary={breakdownSummary}
        open={showBreakdown}
        onToggle={() => setShowBreakdown((prev) => !prev)}
      >
        <div className="grid grid-cols-2 lg:grid-cols-5 gap-3">
          <div className="rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-3">
            <div className="flex items-center justify-between text-xs text-[var(--ink-secondary)]">
              <span>Expected</span>
              <Users className="h-3.5 w-3.5 text-[var(--ink-tertiary)]" />
            </div>
            <div className="text-xl font-bold tabular-nums text-[var(--ink-primary)] mt-1">
              {stats.expected}
            </div>
            <div className="text-[11px] text-[var(--ink-tertiary)] mt-0.5">
              {stats.rate}% completion
            </div>
          </div>

          <div className="rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-3">
            <div className="flex items-center justify-between text-xs text-[var(--ink-secondary)]">
              <span>Checked in</span>
              <UserCheck className="h-3.5 w-3.5 text-[var(--color-emerald)]" />
            </div>
            <div className="text-xl font-bold tabular-nums text-[var(--color-emerald)] mt-1">
              {stats.presentCount}
            </div>
            <div className="text-[11px] text-[var(--ink-tertiary)] mt-0.5 tabular-nums">
              {stats.selfCount} self · {stats.staffCount} staff
            </div>
          </div>

          <div className="rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-3">
            <div className="flex items-center justify-between text-xs text-[var(--ink-secondary)]">
              <span>Not checked in</span>
              <UserX className="h-3.5 w-3.5 text-[var(--color-coral)]" />
            </div>
            <div className="text-xl font-bold tabular-nums text-[var(--color-coral)] mt-1">
              {stats.absentCount}
            </div>
            <div className="text-[11px] text-[var(--ink-tertiary)] mt-0.5">
              Still pending
            </div>
          </div>

          <div className="rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-3">
            <div className="flex items-center justify-between text-xs text-[var(--ink-secondary)]">
              <span>Check-in window</span>
              <Clock className="h-3.5 w-3.5 text-[var(--ink-tertiary)]" />
            </div>
            <div className="text-xs font-mono font-semibold text-[var(--ink-primary)] mt-1.5 space-y-0.5">
              <div className="truncate">
                <span className="text-[var(--ink-tertiary)] font-sans font-normal">
                  First:{" "}
                </span>
                {firstFmt ? firstFmt.time : "—"}
              </div>
              <div className="truncate">
                <span className="text-[var(--ink-tertiary)] font-sans font-normal">
                  Latest:{" "}
                </span>
                {latestFmt ? latestFmt.time : "—"}
              </div>
            </div>
          </div>

          <div className="rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-3 col-span-2 lg:col-span-1">
            <div className="flex items-center justify-between text-xs text-[var(--ink-secondary)]">
              <span>Teams complete</span>
              <CheckCircle2 className="h-3.5 w-3.5 text-[var(--color-blue)]" />
            </div>
            <div className="text-xl font-bold tabular-nums text-[var(--ink-primary)] mt-1">
              {stats.totalTeams > 0
                ? `${stats.completeTeams}/${stats.totalTeams}`
                : "—"}
            </div>
            <div className="text-[11px] text-[var(--ink-tertiary)] mt-0.5">
              {stats.totalTeams > 0 ? "All members present" : "No teams linked"}
            </div>
          </div>
        </div>
      </CollapsibleSection>

      {/* Collapsible: Attendee Roster */}
      <CollapsibleSection
        title="Attendee roster"
        summary={`${people.length} total · ${stats.presentCount} in · ${stats.absentCount} pending`}
        open={showRoster}
        onToggle={() => setShowRoster((prev) => !prev)}
      >
        {/* Roster Filter Bar */}
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-3">
          <div className="inline-flex rounded-lg border border-[var(--card-line)] p-1 bg-[var(--card-solid-subtle)] gap-1 self-start">
            {(
              [
                ["all", `All (${people.length})`],
                ["present", `Checked in (${stats.presentCount})`],
                ["absent", `Pending (${stats.absentCount})`],
              ] as const
            ).map(([key, label]) => (
              <button
                key={key}
                type="button"
                onClick={() => setView(key)}
                className={cn(
                  "px-2.5 py-1 text-xs rounded-[6px] transition-colors cursor-pointer",
                  view === key
                    ? "bg-[var(--card-solid)] text-[var(--ink-primary)] shadow-xs border border-[var(--card-line)] font-semibold"
                    : "text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] font-medium"
                )}
              >
                {label}
              </button>
            ))}
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <div className="relative flex-1 min-w-[200px]">
              <Search className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-[var(--ink-tertiary)]" />
              <Input
                placeholder="Search name, email, team, phone..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="h-8 pl-8 text-xs"
              />
            </div>

            {uniqueTeams.length > 0 && (
              <div className="w-36">
                <Select
                  aria-label="Filter by team"
                  value={teamFilter}
                  onChange={(e) => setTeamFilter(e.target.value)}
                  className="h-8 text-xs"
                >
                  <option value="">All teams</option>
                  <option value="__none__">No team</option>
                  {uniqueTeams.map((t) => (
                    <option key={t} value={t}>
                      {t}
                    </option>
                  ))}
                </Select>
              </div>
            )}

            <div className="w-40">
              <Select
                aria-label="Sort attendees"
                value={sortBy}
                onChange={(e) => setSortBy(e.target.value as RosterSort)}
                className="h-8 text-xs"
              >
                <option value="latest">Latest first</option>
                <option value="earliest">Earliest first</option>
                <option value="name">Name (A–Z)</option>
                <option value="team">Team (A–Z)</option>
              </Select>
            </div>
          </div>
        </div>

        {/* Detailed Roster Table */}
        {filteredPeople.length === 0 ? (
          <EmptyState
            title="No matching attendees"
            description="Try clearing the search or switching between Checked in and Pending."
          />
        ) : (
          <div className="max-h-[420px] overflow-auto rounded-[12px] border border-[var(--card-line)]">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Attendee</TableHead>
                  <TableHead>Team</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>When</TableHead>
                  <TableHead>Checked in by</TableHead>
                  <TableHead>Phone</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredPeople.map((p) => {
                  const ts = formatTimestamp(p.scanned_at);
                  return (
                    <TableRow key={p.user_id}>
                      <TableCell>
                        <div className="flex items-center gap-1.5">
                          <span className="font-semibold text-[var(--ink-primary)] text-xs">
                            {p.name}
                          </span>
                          <Badge variant="subtle" size="sm">
                            {p.is_external ? "External" : "Internal"}
                          </Badge>
                        </div>
                        <div className="text-[11px] text-[var(--ink-tertiary)] font-mono">
                          {p.email}
                        </div>
                      </TableCell>
                      <TableCell className="text-xs">
                        {p.team_name ? (
                          <div className="flex items-center gap-1.5">
                            <span className="font-medium text-[var(--ink-primary)]">
                              {p.team_name}
                            </span>
                            {p.is_leader && (
                              <Badge variant="blue" size="sm">
                                Lead
                              </Badge>
                            )}
                          </div>
                        ) : (
                          <span className="text-[var(--ink-tertiary)]">—</span>
                        )}
                      </TableCell>
                      <TableCell>
                        <Badge
                          variant={p.present ? "emerald" : "coral"}
                          dot
                          size="sm"
                        >
                          {p.present ? "Checked in" : "Pending"}
                        </Badge>
                      </TableCell>
                      <TableCell className="font-mono text-xs tabular-nums">
                        {ts ? (
                          <div>
                            <div className="text-[var(--ink-primary)]">
                              {ts.time}
                            </div>
                            <div className="text-[10px] text-[var(--ink-tertiary)]">
                              {ts.date}
                            </div>
                          </div>
                        ) : (
                          <span className="text-[var(--ink-tertiary)]">—</span>
                        )}
                      </TableCell>
                      <TableCell className="text-xs text-[var(--ink-secondary)]">
                        {formatCheckedInBy(p)}
                      </TableCell>
                      <TableCell className="font-mono text-xs text-[var(--ink-secondary)]">
                        {p.phone_number || "—"}
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </div>
        )}
      </CollapsibleSection>
    </div>
  );
}

function TokenCard({
  session,
  isSelected,
  isBusy,
  isManageOpen,
  onSelect,
  onToggleManage,
  onCopyToken,
  onEditCustom,
  onRotateRandom,
  onDisableCode,
  onToggleOpen,
}: {
  session: ScanSession;
  isSelected: boolean;
  isBusy: boolean;
  isManageOpen: boolean;
  onSelect: () => void;
  onToggleManage: () => void;
  onCopyToken: (code: string) => void;
  onEditCustom: () => void;
  onRotateRandom: () => void;
  onDisableCode: () => void;
  onToggleOpen: () => void;
}) {
  return (
    <div
      onClick={onSelect}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onSelect();
        }
      }}
      className={cn(
        "rounded-[14px] border p-3.5 transition-colors cursor-pointer space-y-2.5 text-left",
        isSelected
          ? "border-[var(--color-blue)] bg-[var(--color-blue-dim)]/35 ring-1 ring-[var(--color-blue)]/30"
          : "border-[var(--card-line)] bg-[var(--card-solid-subtle)]/50 hover:bg-[var(--card-solid-subtle)]"
      )}
    >
      {/* Top row: Name + status */}
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-1.5">
            <span className="text-sm font-semibold text-[var(--ink-primary)] truncate">
              {session.name}
            </span>
            {session.is_default && (
              <Badge variant="blue" size="sm">
                Default
              </Badge>
            )}
          </div>
          <div className="text-[11px] text-[var(--ink-secondary)] capitalize mt-0.5 truncate">
            {session.kind === "check_in" ? "Entry check-in" : session.kind}
            {session.competition_name ? ` · ${session.competition_name}` : ""}
          </div>
        </div>

        <Badge variant={session.is_open ? "emerald" : "subtle"} size="sm" dot>
          {session.is_open ? "Open" : "Closed"}
        </Badge>
      </div>

      {/* Primary row: Token + Copy | Count + Manage toggle */}
      <div
        className="flex items-center justify-between gap-2 pt-2 border-t border-[var(--card-line)]"
        onClick={(e) => e.stopPropagation()}
        onKeyDown={(e) => e.stopPropagation()}
      >
        {session.checkin_code ? (
          <button
            type="button"
            onClick={() => onCopyToken(session.checkin_code!)}
            title="Copy check-in token"
            className="inline-flex items-center gap-1.5 font-mono text-xs font-bold tracking-wider px-2.5 py-1 rounded-[6px] border border-[var(--color-blue-border)] bg-[var(--card-solid)] text-[var(--color-blue)] hover:opacity-90 min-w-0 cursor-pointer"
          >
            <span className="truncate">{session.checkin_code}</span>
            <Copy className="h-3 w-3 shrink-0" />
          </button>
        ) : (
          <Button
            variant="outline"
            size="sm"
            disabled={isBusy}
            onClick={onRotateRandom}
            className="h-7 px-2.5 text-[11px]"
          >
            Generate token
          </Button>
        )}

        <div className="flex items-center gap-1.5 shrink-0">
          <span className="text-xs font-mono font-semibold text-[var(--ink-primary)] tabular-nums">
            {session.scanned_count}
          </span>
          <span className="text-[11px] text-[var(--ink-tertiary)]">in</span>
          <button
            type="button"
            onClick={onToggleManage}
            aria-expanded={isManageOpen}
            title="Manage token"
            className={cn(
              "p-1.5 rounded-[6px] transition-colors cursor-pointer",
              isManageOpen
                ? "bg-[var(--card-solid)] text-[var(--ink-primary)] border border-[var(--card-line)]"
                : "text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] hover:bg-[var(--card-solid)]"
            )}
          >
            <MoreHorizontal className="h-3.5 w-3.5" />
          </button>
        </div>
      </div>

      {/* Collapsible secondary actions */}
      {isManageOpen && (
        <div
          className="flex flex-wrap items-center justify-between gap-1.5 pt-2 border-t border-[var(--card-line)]"
          onClick={(e) => e.stopPropagation()}
          onKeyDown={(e) => e.stopPropagation()}
        >
          <div className="flex flex-wrap items-center gap-1">
            <Button
              variant="ghost"
              size="sm"
              disabled={isBusy}
              onClick={onEditCustom}
              className="h-7 px-2 text-[11px] gap-1"
            >
              <Pencil className="h-3 w-3" />
              <span>Custom</span>
            </Button>
            <Button
              variant="ghost"
              size="sm"
              disabled={isBusy}
              onClick={onRotateRandom}
              className="h-7 px-2 text-[11px] gap-1"
            >
              <RefreshCw
                className={`h-3 w-3 ${isBusy ? "animate-spin" : ""}`}
              />
              <span>Rotate</span>
            </Button>
            {session.checkin_code && (
              <Button
                variant="ghost"
                size="sm"
                disabled={isBusy}
                onClick={onDisableCode}
                className="h-7 px-2 text-[11px] gap-1 text-[var(--color-coral)] hover:bg-[var(--color-coral-dim)]"
              >
                <X className="h-3 w-3" />
                <span>Disable</span>
              </Button>
            )}
          </div>

          <Button
            variant="outline"
            size="sm"
            className="h-7 px-2.5 text-[11px]"
            onClick={onToggleOpen}
          >
            {session.is_open ? "Close" : "Reopen"}
          </Button>
        </div>
      )}
    </div>
  );
}

export function CheckInTokensPanel() {
  const toast = useToast();
  const { competitions, activeCompetition } = useTenant();
  const { data: sessions = [], isLoading } = useScanSessions();
  const create = useCreateScanSession();
  const setOpen = useSetScanSessionOpen();
  const generateCode = useGenerateSessionCode();
  const clearCode = useClearSessionCode();

  // Collapsible UI state
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [showClosed, setShowClosed] = useState(false);
  const [manageOpenId, setManageOpenId] = useState<string | null>(null);

  // Create new token form state
  const [name, setName] = useState("");
  const [customToken, setCustomToken] = useState("");
  const [kind, setKind] = useState<ScanKind>("check_in");
  const [markTeam, setMarkTeam] = useState(false);
  const [selectedCompetitionId, setSelectedCompetitionId] = useState("");

  // Selected session for detailed stats
  const [selectedSessionId, setSelectedSessionId] = useState<string | null>(
    null
  );

  // Edit custom token dialog
  const [editingSession, setEditingSession] = useState<ScanSession | null>(
    null
  );
  const [customCodeDraft, setCustomCodeDraft] = useState("");
  const [busySessionId, setBusySessionId] = useState<string | null>(null);

  const openSessions = useMemo(
    () => sessions.filter((s) => s.is_open),
    [sessions]
  );
  const closedSessions = useMemo(
    () => sessions.filter((s) => !s.is_open),
    [sessions]
  );

  const effectiveCompetitionId =
    selectedCompetitionId ||
    (kind === "attendance" && markTeam
      ? activeCompetition?.id || competitions[0]?.id || ""
      : "");

  const selectedSession = useMemo(() => {
    if (sessions.length === 0) return null;
    if (selectedSessionId) {
      const found = sessions.find((s) => s.id === selectedSessionId);
      if (found) return found;
    }
    return openSessions[0] ?? sessions[0];
  }, [sessions, openSessions, selectedSessionId]);

  const handleCreateToken = async (e: React.FormEvent) => {
    e.preventDefault();
    const built = buildCreateScanSessionPayload({
      name,
      kind,
      markTeam: kind === "attendance" ? markTeam : false,
      competitionId: kind === "attendance" ? effectiveCompetitionId : "",
      enableCode: true,
      customCode: customToken,
    });
    if (built.error !== undefined) {
      toast.error(built.error, "Invalid token configuration");
      return;
    }

    try {
      const created = await create.mutateAsync(built.payload);
      setName("");
      setCustomToken("");
      setShowCreateForm(false);
      setSelectedSessionId(created.id);
      toast.success(
        created.checkin_code
          ? `Created "${created.name}" with token ${created.checkin_code}.`
          : `Created "${created.name}".`,
        "Check-in token created"
      );
    } catch (err: unknown) {
      toast.error(
        apiErrorMessage(err, "Failed to create check-in token"),
        "Error"
      );
    }
  };

  const handleRotateOrSetCode = async (s: ScanSession, custom?: string) => {
    const codeErr = validateCustomCheckInCode(custom);
    if (codeErr) {
      toast.error(codeErr, "Invalid token");
      return;
    }
    setBusySessionId(s.id);
    try {
      const normalized = normalizeCheckInCode(custom);
      const updated = await generateCode.mutateAsync({
        sessionId: s.id,
        code: normalized || undefined,
      });
      setEditingSession(null);
      setCustomCodeDraft("");
      toast.success(
        `Active token for "${s.name}": ${updated.checkin_code}`,
        s.checkin_code ? "Token updated" : "Token generated"
      );
    } catch (err: unknown) {
      toast.error(apiErrorMessage(err, "Failed to update token"), "Error");
    } finally {
      setBusySessionId(null);
    }
  };

  const handleDisableCode = async (s: ScanSession) => {
    setBusySessionId(s.id);
    try {
      await clearCode.mutateAsync(s.id);
      toast.info(`Token disabled for "${s.name}".`, "Token disabled");
    } catch (err: unknown) {
      toast.error(apiErrorMessage(err, "Failed to disable token"), "Error");
    } finally {
      setBusySessionId(null);
    }
  };

  const handleToggleOpen = async (s: ScanSession) => {
    try {
      await setOpen.mutateAsync({ id: s.id, isOpen: !s.is_open });
      toast.info(
        s.is_open ? `"${s.name}" closed.` : `"${s.name}" reopened.`,
        s.is_open ? "Check-in closed" : "Check-in open"
      );
    } catch (err: unknown) {
      toast.error(
        apiErrorMessage(err, "Failed to toggle check-in state"),
        "Error"
      );
    }
  };

  const copyToken = async (code: string) => {
    try {
      await navigator.clipboard.writeText(code);
      toast.success(`Token ${code} copied.`, "Copied");
    } catch {
      toast.info(`Token: ${code}`, "Copy manually");
    }
  };

  const renderCard = (s: ScanSession) => (
    <TokenCard
      key={s.id}
      session={s}
      isSelected={selectedSession?.id === s.id}
      isBusy={busySessionId === s.id}
      isManageOpen={manageOpenId === s.id}
      onSelect={() => setSelectedSessionId(s.id)}
      onToggleManage={() =>
        setManageOpenId((prev) => (prev === s.id ? null : s.id))
      }
      onCopyToken={copyToken}
      onEditCustom={() => {
        setEditingSession(s);
        setCustomCodeDraft(s.checkin_code || "");
      }}
      onRotateRandom={() => handleRotateOrSetCode(s)}
      onDisableCode={() => handleDisableCode(s)}
      onToggleOpen={() => handleToggleOpen(s)}
    />
  );

  return (
    <div className="space-y-5 font-sans">
      {/* Tokens Card (with collapsible Create form) */}
      <div className={card}>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <h2 className="text-sm font-semibold text-[var(--ink-primary)]">
              Check-in tokens
            </h2>
            <Badge variant="subtle" size="sm">
              {openSessions.length} open
            </Badge>
          </div>

          <Button
            type="button"
            variant={showCreateForm ? "outline" : "primary"}
            size="sm"
            onClick={() => setShowCreateForm((prev) => !prev)}
            className="h-8 px-3 text-xs gap-1.5"
          >
            {showCreateForm ? (
              <>
                <X className="h-3.5 w-3.5" />
                <span>Cancel</span>
              </>
            ) : (
              <>
                <Plus className="h-3.5 w-3.5" />
                <span>New token</span>
              </>
            )}
          </Button>
        </div>

        {/* Collapsible Create Token Form */}
        {showCreateForm && (
          <form
            onSubmit={handleCreateToken}
            className="rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)]/50 p-3.5 space-y-3"
          >
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-2.5 items-end">
              <Input
                label="Name *"
                placeholder="e.g. Day 1 Entry, Lunch"
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="text-xs h-9"
                autoFocus
              />

              <Input
                label="Custom token (optional)"
                placeholder="Blank = random"
                value={customToken}
                maxLength={32}
                onChange={(e) => setCustomToken(e.target.value.toUpperCase())}
                className="text-xs h-9 font-mono uppercase tracking-wider"
              />

              <Select
                label="Type"
                value={kind}
                onChange={(e) => setKind(e.target.value as ScanKind)}
                className="text-xs h-9"
              >
                <option value="check_in">Entry check-in</option>
                <option value="attendance">Attendance round</option>
                <option value="exit">Exit</option>
              </Select>

              <Button
                type="submit"
                variant="primary"
                size="sm"
                disabled={
                  name.trim().length < 2 ||
                  create.isPending ||
                  (kind === "attendance" && markTeam && !effectiveCompetitionId)
                }
                isLoading={create.isPending}
                className="h-9 text-xs gap-1.5 font-semibold"
              >
                <Plus className="h-3.5 w-3.5" />
                Create token
              </Button>
            </div>

            {kind === "attendance" && (
              <div className="flex flex-wrap items-center gap-4 pt-1">
                <div className="w-60">
                  <Select
                    aria-label="Competition scope"
                    value={effectiveCompetitionId}
                    onChange={(e) => setSelectedCompetitionId(e.target.value)}
                    className="text-xs h-9"
                  >
                    {!markTeam && (
                      <option value="">All event members</option>
                    )}
                    {competitions.map((comp) => (
                      <option key={comp.id} value={comp.id}>
                        Competition: {comp.name}
                      </option>
                    ))}
                  </Select>
                </div>

                <label className="flex items-center gap-2 text-xs cursor-pointer select-none text-[var(--ink-primary)]">
                  <input
                    type="checkbox"
                    checked={markTeam}
                    onChange={(e) => setMarkTeam(e.target.checked)}
                    className="accent-[var(--color-blue)] h-4 w-4 rounded"
                  />
                  <span>Team leader check-in marks whole team</span>
                </label>
              </div>
            )}
          </form>
        )}

        {/* Open & Closed Tokens */}
        {isLoading ? (
          <SkeletonCard />
        ) : sessions.length === 0 ? (
          <EmptyState
            title="No check-in tokens yet"
            description="Create a token to let attendees check in with a short code."
            action={
              !showCreateForm ? (
                <Button
                  type="button"
                  variant="primary"
                  size="sm"
                  onClick={() => setShowCreateForm(true)}
                  className="text-xs gap-1.5"
                >
                  <Plus className="h-3.5 w-3.5" />
                  Create check-in token
                </Button>
              ) : undefined
            }
          />
        ) : (
          <div className="space-y-3">
            {openSessions.length > 0 ? (
              <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3">
                {openSessions.map(renderCard)}
              </div>
            ) : (
              <p className="text-xs text-[var(--ink-secondary)]">
                No open check-in tokens right now.
              </p>
            )}

            {closedSessions.length > 0 && (
              <CollapsibleSection
                title="Closed tokens"
                summary={`${closedSessions.length} closed`}
                open={showClosed || openSessions.length === 0}
                onToggle={() => setShowClosed((prev) => !prev)}
              >
                <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3">
                  {closedSessions.map(renderCard)}
                </div>
              </CollapsibleSection>
            )}
          </div>
        )}
      </div>

      {/* Selected Token Summary + Collapsible Details & Roster */}
      {selectedSession && (
        <SessionDetailedStats
          key={selectedSession.id}
          session={selectedSession}
          onCopyToken={copyToken}
        />
      )}

      {/* Custom Token Dialog */}
      <Dialog
        open={Boolean(editingSession)}
        onClose={() => {
          setEditingSession(null);
          setCustomCodeDraft("");
        }}
        title={
          editingSession
            ? `Custom token · ${editingSession.name}`
            : "Set custom token"
        }
        description="Enter a custom check-in token (2–32 letters, numbers, hyphens, or underscores). Leave blank to generate a random 6-character token."
        footer={
          <div className="flex justify-end gap-2">
            <Button
              variant="outline"
              size="sm"
              type="button"
              onClick={() => {
                setEditingSession(null);
                setCustomCodeDraft("");
              }}
            >
              Cancel
            </Button>
            <Button
              variant="primary"
              size="sm"
              type="button"
              isLoading={Boolean(
                editingSession && busySessionId === editingSession.id
              )}
              onClick={() => {
                if (editingSession) {
                  void handleRotateOrSetCode(editingSession, customCodeDraft);
                }
              }}
            >
              Save token
            </Button>
          </div>
        }
      >
        <form
          onSubmit={(e) => {
            e.preventDefault();
            if (editingSession) {
              void handleRotateOrSetCode(editingSession, customCodeDraft);
            }
          }}
          className="space-y-2"
        >
          <Input
            label="Check-in token"
            placeholder="e.g. DAY1-IN (blank for random)"
            value={customCodeDraft}
            maxLength={32}
            onChange={(e) => setCustomCodeDraft(e.target.value.toUpperCase())}
            className="font-mono uppercase tracking-wider text-xs"
            autoFocus
          />
        </form>
      </Dialog>
    </div>
  );
}
