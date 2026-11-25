"use client";

import React, { useMemo, useState } from "react";
import {
  CheckCircle2,
  Clock,
  Copy,
  Download,
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

function formatTimestamp(iso?: string | null): { date: string; time: string; full: string } | null {
  if (!iso) return null;
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return null;
  const date = d.toLocaleDateString([], { month: "short", day: "numeric" });
  const time = d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
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

function SessionDetailedStats({ session }: { session: ScanSession }) {
  const { data, isLoading, isFetching, refetch } = useSessionReport(session.id);

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

  return (
    <div className={card}>
      {/* Header & CSV Exports */}
      <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-3 pb-3 border-b border-[var(--card-line)]">
        <div className="space-y-1">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="text-base font-bold tracking-tight text-[var(--ink-primary)]">
              {session.name}
            </h3>
            {session.checkin_code && (
              <span className="font-mono text-xs font-semibold tracking-wider px-2 py-0.5 rounded-[6px] border border-[var(--color-blue-border)] bg-[var(--color-blue-dim)] text-[var(--color-blue)]">
                {session.checkin_code}
              </span>
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
          <p className="text-xs text-[var(--ink-secondary)]">
            Detailed attendance breakdown — who checked in, exact timestamps, who hasn&apos;t checked in yet, and CSV exports.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => refetch()}
            disabled={isFetching}
            className="h-8 px-2.5 text-xs gap-1.5"
          >
            <RefreshCw className={`h-3.5 w-3.5 ${isFetching ? "animate-spin" : ""}`} />
            Refresh
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => exportCsv("present")}
            disabled={stats.presentCount === 0}
            className="h-8 px-2.5 text-xs gap-1.5"
          >
            <Download className="h-3.5 w-3.5" />
            Who checked in ({stats.presentCount})
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => exportCsv("absent")}
            disabled={stats.absentCount === 0}
            className="h-8 px-2.5 text-xs gap-1.5"
          >
            <Download className="h-3.5 w-3.5" />
            Who didn&apos;t ({stats.absentCount})
          </Button>
          <Button
            variant="primary"
            size="sm"
            onClick={() => exportCsv("all")}
            disabled={people.length === 0}
            className="h-8 px-3 text-xs gap-1.5 font-semibold"
          >
            <Download className="h-3.5 w-3.5" />
            Export all CSV
          </Button>
        </div>
      </div>

      {/* KPI Metrics Grid */}
      <div className="grid grid-cols-2 lg:grid-cols-5 gap-3">
        <div className="rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-3.5">
          <div className="flex items-center justify-between text-xs text-[var(--ink-secondary)]">
            <span>Expected</span>
            <Users className="h-3.5 w-3.5 text-[var(--ink-tertiary)]" />
          </div>
          <div className="text-2xl font-bold tabular-nums text-[var(--ink-primary)] mt-1">
            {stats.expected}
          </div>
          <div className="text-[11px] text-[var(--ink-tertiary)] mt-0.5">
            {stats.rate}% completion
          </div>
        </div>

        <div className="rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-3.5">
          <div className="flex items-center justify-between text-xs text-[var(--ink-secondary)]">
            <span>Checked in</span>
            <UserCheck className="h-3.5 w-3.5 text-[var(--color-emerald)]" />
          </div>
          <div className="text-2xl font-bold tabular-nums text-[var(--color-emerald)] mt-1">
            {stats.presentCount}
          </div>
          <div className="text-[11px] text-[var(--ink-tertiary)] mt-0.5 tabular-nums">
            {stats.selfCount} self · {stats.staffCount} staff
          </div>
        </div>

        <div className="rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-3.5">
          <div className="flex items-center justify-between text-xs text-[var(--ink-secondary)]">
            <span>Not checked in</span>
            <UserX className="h-3.5 w-3.5 text-[var(--color-coral)]" />
          </div>
          <div className="text-2xl font-bold tabular-nums text-[var(--color-coral)] mt-1">
            {stats.absentCount}
          </div>
          <div className="text-[11px] text-[var(--ink-tertiary)] mt-0.5">
            Still pending
          </div>
        </div>

        <div className="rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-3.5">
          <div className="flex items-center justify-between text-xs text-[var(--ink-secondary)]">
            <span>Check-in window</span>
            <Clock className="h-3.5 w-3.5 text-[var(--ink-tertiary)]" />
          </div>
          <div className="text-xs font-mono font-semibold text-[var(--ink-primary)] mt-1.5 space-y-0.5">
            <div className="truncate">
              <span className="text-[var(--ink-tertiary)] font-sans font-normal">First: </span>
              {firstFmt ? firstFmt.time : "—"}
            </div>
            <div className="truncate">
              <span className="text-[var(--ink-tertiary)] font-sans font-normal">Latest: </span>
              {latestFmt ? latestFmt.time : "—"}
            </div>
          </div>
        </div>

        <div className="rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-3.5 col-span-2 lg:col-span-1">
          <div className="flex items-center justify-between text-xs text-[var(--ink-secondary)]">
            <span>Teams complete</span>
            <CheckCircle2 className="h-3.5 w-3.5 text-[var(--color-blue)]" />
          </div>
          <div className="text-2xl font-bold tabular-nums text-[var(--ink-primary)] mt-1">
            {stats.totalTeams > 0 ? `${stats.completeTeams}/${stats.totalTeams}` : "—"}
          </div>
          <div className="text-[11px] text-[var(--ink-tertiary)] mt-0.5">
            {stats.totalTeams > 0 ? "All members present" : "No teams linked"}
          </div>
        </div>
      </div>

      {/* Progress bar */}
      <div className="space-y-1">
        <div className="h-2 w-full overflow-hidden rounded-full bg-[var(--card-solid-subtle)]">
          <div
            className="h-full rounded-full bg-[var(--color-emerald)] transition-[width] duration-300"
            style={{ width: `${Math.min(stats.rate, 100)}%` }}
          />
        </div>
      </div>

      {/* Roster Filter Bar */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-3">
        <div className="inline-flex rounded-lg border border-[var(--card-line)] p-1 bg-[var(--card-solid-subtle)] gap-1 self-start">
          {(
            [
              ["all", `All (${people.length})`],
              ["present", `Checked in (${stats.presentCount})`],
              ["absent", `Not checked in (${stats.absentCount})`],
            ] as const
          ).map(([key, label]) => (
            <button
              key={key}
              type="button"
              onClick={() => setView(key)}
              className={cn(
                "px-3 py-1 text-xs rounded-[6px] transition-colors cursor-pointer",
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
            <div className="w-40">
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

          <div className="w-44">
            <Select
              aria-label="Sort attendees"
              value={sortBy}
              onChange={(e) => setSortBy(e.target.value as RosterSort)}
              className="h-8 text-xs"
            >
              <option value="latest">Latest check-in first</option>
              <option value="earliest">Earliest check-in first</option>
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
          description="Try clearing the search or switching between Checked in and Not checked in."
        />
      ) : (
        <div className="max-h-[460px] overflow-auto rounded-[12px] border border-[var(--card-line)]">
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
                        {p.present ? "Checked in" : "Not checked in"}
                      </Badge>
                    </TableCell>
                    <TableCell className="font-mono text-xs tabular-nums">
                      {ts ? (
                        <div>
                          <div className="text-[var(--ink-primary)]">{ts.time}</div>
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

  // Create new token form state
  const [name, setName] = useState("");
  const [customToken, setCustomToken] = useState("");
  const [kind, setKind] = useState<ScanKind>("check_in");
  const [markTeam, setMarkTeam] = useState(false);
  const [selectedCompetitionId, setSelectedCompetitionId] = useState("");

  // Selected session for detailed stats
  const [selectedSessionId, setSelectedSessionId] = useState<string | null>(null);

  // Edit custom token dialog
  const [editingSession, setEditingSession] = useState<ScanSession | null>(null);
  const [customCodeDraft, setCustomCodeDraft] = useState("");
  const [busySessionId, setBusySessionId] = useState<string | null>(null);

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
    return sessions[0];
  }, [sessions, selectedSessionId]);

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
      setSelectedSessionId(created.id);
      toast.success(
        created.checkin_code
          ? `Created "${created.name}" with token ${created.checkin_code}.`
          : `Created "${created.name}".`,
        "Check-in token created"
      );
    } catch (err: unknown) {
      toast.error(apiErrorMessage(err, "Failed to create check-in token"), "Error");
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
      toast.error(apiErrorMessage(err, "Failed to toggle check-in state"), "Error");
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

  return (
    <div className="space-y-6 font-sans">
      {/* Create Check-in Token Form */}
      <form onSubmit={handleCreateToken} className={card}>
        <div>
          <h2 className="text-sm font-semibold text-[var(--ink-primary)]">
            Create check-in token
          </h2>
          <p className="text-xs text-[var(--ink-secondary)] mt-0.5">
            Create multiple check-in tokens (e.g. Day 1 Entry, Lunch, Midnight Check) with a custom code or auto-generated token.
          </p>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-2.5 items-end">
          <Input
            label="Check-in name *"
            placeholder="e.g. Day 1 Entry, Dinner Check-in"
            value={name}
            onChange={(e) => setName(e.target.value)}
            className="text-xs h-9"
          />

          <Input
            label="Custom token (optional)"
            placeholder="Blank = random (e.g. DAY1-IN)"
            value={customToken}
            maxLength={32}
            onChange={(e) => setCustomToken(e.target.value.toUpperCase())}
            className="text-xs h-9 font-mono uppercase tracking-wider"
          />

          <Select
            label="Check-in type"
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
                  <option value="">All event members (no competition)</option>
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

      {/* Active & Past Check-in Tokens List */}
      <div className={card}>
        <div className="flex items-center justify-between gap-2">
          <div>
            <h2 className="text-sm font-semibold text-[var(--ink-primary)]">
              Check-in tokens
            </h2>
            <p className="text-xs text-[var(--ink-secondary)] mt-0.5">
              Select any token to inspect detailed who/when/who-not attendance stats and export CSVs.
            </p>
          </div>
          <Badge variant="subtle" size="sm">
            {sessions.length} {sessions.length === 1 ? "token" : "tokens"}
          </Badge>
        </div>

        {isLoading ? (
          <SkeletonCard />
        ) : sessions.length === 0 ? (
          <EmptyState
            title="No check-in tokens yet"
            description="Create your first check-in token above."
          />
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3">
            {sessions.map((s) => {
              const isSelected = selectedSession?.id === s.id;
              const isBusy = busySessionId === s.id;
              return (
                <div
                  key={s.id}
                  onClick={() => setSelectedSessionId(s.id)}
                  role="button"
                  tabIndex={0}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      setSelectedSessionId(s.id);
                    }
                  }}
                  className={cn(
                    "rounded-[14px] border p-3.5 transition-colors cursor-pointer space-y-3 text-left",
                    isSelected
                      ? "border-[var(--color-blue)] bg-[var(--color-blue-dim)]/35 ring-1 ring-[var(--color-blue)]/30"
                      : "border-[var(--card-line)] bg-[var(--card-solid-subtle)]/50 hover:bg-[var(--card-solid-subtle)]"
                  )}
                >
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-1.5">
                        <span className="text-sm font-semibold text-[var(--ink-primary)] truncate">
                          {s.name}
                        </span>
                        {s.is_default && (
                          <Badge variant="blue" size="sm">
                            Default
                          </Badge>
                        )}
                      </div>
                      <div className="text-[11px] text-[var(--ink-secondary)] capitalize mt-0.5">
                        {s.kind === "check_in" ? "Entry check-in" : s.kind}
                        {s.competition_name ? ` · ${s.competition_name}` : ""}
                      </div>
                    </div>

                    <Badge variant={s.is_open ? "emerald" : "subtle"} size="sm" dot>
                      {s.is_open ? "Open" : "Closed"}
                    </Badge>
                  </div>

                  {/* Token Display & Actions */}
                  <div
                    className="flex items-center justify-between gap-2 pt-2 border-t border-[var(--card-line)]"
                    onClick={(e) => e.stopPropagation()}
                    onKeyDown={(e) => e.stopPropagation()}
                  >
                    {s.checkin_code ? (
                      <div className="flex items-center gap-1 min-w-0">
                        <span className="font-mono text-xs font-bold tracking-wider px-2 py-1 rounded-[6px] border border-[var(--color-blue-border)] bg-[var(--card-solid)] text-[var(--color-blue)] truncate">
                          {s.checkin_code}
                        </span>
                        <button
                          type="button"
                          onClick={() => copyToken(s.checkin_code!)}
                          title="Copy token"
                          className="p-1 rounded-[6px] text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] hover:bg-[var(--card-solid)] cursor-pointer"
                        >
                          <Copy className="h-3.5 w-3.5" />
                        </button>
                        <button
                          type="button"
                          disabled={isBusy}
                          onClick={() => {
                            setEditingSession(s);
                            setCustomCodeDraft(s.checkin_code || "");
                          }}
                          title="Edit custom token"
                          className="p-1 rounded-[6px] text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] hover:bg-[var(--card-solid)] disabled:opacity-50 cursor-pointer"
                        >
                          <Pencil className="h-3.5 w-3.5" />
                        </button>
                        <button
                          type="button"
                          disabled={isBusy}
                          onClick={() => handleRotateOrSetCode(s)}
                          title="Rotate to random token"
                          className="p-1 rounded-[6px] text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] hover:bg-[var(--card-solid)] disabled:opacity-50 cursor-pointer"
                        >
                          <RefreshCw className={`h-3.5 w-3.5 ${isBusy ? "animate-spin" : ""}`} />
                        </button>
                        <button
                          type="button"
                          disabled={isBusy}
                          onClick={() => handleDisableCode(s)}
                          title="Disable token"
                          className="p-1 rounded-[6px] text-[var(--ink-tertiary)] hover:text-[var(--color-coral)] hover:bg-[var(--color-coral-dim)] disabled:opacity-50 cursor-pointer"
                        >
                          <X className="h-3.5 w-3.5" />
                        </button>
                      </div>
                    ) : (
                      <div className="flex items-center gap-1.5">
                        <Button
                          variant="outline"
                          size="sm"
                          disabled={isBusy}
                          onClick={() => handleRotateOrSetCode(s)}
                          className="h-7 px-2 text-[11px]"
                        >
                          Random
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          disabled={isBusy}
                          onClick={() => {
                            setEditingSession(s);
                            setCustomCodeDraft("");
                          }}
                          className="h-7 px-2 text-[11px]"
                        >
                          Custom…
                        </Button>
                      </div>
                    )}

                    <div className="flex items-center gap-2 shrink-0">
                      <span className="text-xs font-mono font-semibold text-[var(--ink-primary)] tabular-nums">
                        {s.scanned_count}
                      </span>
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-7 px-2 text-[11px]"
                        onClick={() => handleToggleOpen(s)}
                      >
                        {s.is_open ? "Close" : "Reopen"}
                      </Button>
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* Selected Token Detailed Stats & Roster */}
      {selectedSession && (
        <SessionDetailedStats key={selectedSession.id} session={selectedSession} />
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
