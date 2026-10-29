"use client";

import React, { useEffect } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  ArrowRight,
  ArrowUpRight,
  Bell,
  Award,
  QrCode,
  Users,
  Layers,
  FileText,
} from "lucide-react";
import { useAuth } from "@/providers/auth-provider";
import { getDashboardRoute } from "@/lib/auth-routes";
import { attendancePercent } from "@/lib/utils";
import { useTicketAnalytics } from "@/hooks/use-tickets";
import { useUsers } from "@/hooks/use-users";
import { useTeams } from "@/hooks/use-teams";
import { useJudgingRounds, useAdminLeaderboard } from "@/hooks/use-judging";
import { useAuditLogs } from "@/hooks/use-audit";
import { Skeleton, SkeletonCard } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";

const timeAgo = (iso: string) => {
  const s = Math.max(1, Math.floor((Date.now() - new Date(iso).getTime()) / 1000));
  if (s < 60) return `${s}s ago`;
  if (s < 3600) return `${Math.floor(s / 60)}m ago`;
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`;
  return `${Math.floor(s / 86400)}d ago`;
};

function Panel({
  title,
  action,
  children,
  className = "",
}: {
  title: string;
  action?: { label: string; href: string };
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <section
      className={`rounded-[16px] border border-[var(--card-line)] bg-[var(--card-solid)] ${className}`}
    >
      <header className="flex items-center justify-between px-5 pt-4 pb-3">
        <h2 className="text-sm font-semibold tracking-tight text-[var(--ink-primary)]">{title}</h2>
        {action && (
          <Link
            href={action.href}
            className="inline-flex items-center gap-1 text-xs font-medium text-[var(--color-blue)] hover:underline"
          >
            {action.label}
            <ArrowRight className="h-3 w-3" />
          </Link>
        )}
      </header>
      <div className="px-5 pb-5">{children}</div>
    </section>
  );
}

function Stat({
  label,
  value,
  note,
  href,
  icon: Icon,
}: {
  label: string;
  value: React.ReactNode;
  note?: string;
  href: string;
  icon: React.ComponentType<{ className?: string }>;
}) {
  return (
    <Link
      href={href}
      className="group rounded-[16px] border border-[var(--card-line)] bg-[var(--card-solid)] p-4 transition-[border-color,transform] duration-150 hover:border-[var(--color-blue-border)] active:scale-[0.98]"
    >
      <div className="flex items-center justify-between text-[var(--ink-tertiary)]">
        <span className="text-xs font-medium text-[var(--ink-secondary)]">{label}</span>
        <Icon className="h-4 w-4 transition-colors group-hover:text-[var(--color-blue)]" />
      </div>
      <div className="mt-2 text-2xl font-bold tabular-nums tracking-tight text-[var(--ink-primary)]">
        {value}
      </div>
      {note && <div className="mt-0.5 text-[11px] text-[var(--ink-secondary)] truncate">{note}</div>}
    </Link>
  );
}

export default function AdminDashboardPage() {
  const router = useRouter();
  const { user, isAuthenticated, isLoading } = useAuth();
  const isAdmin = user?.role === "admin";

  useEffect(() => {
    if (isLoading) return;
    if (!isAuthenticated || !user) router.replace("/login");
    else if (user.role !== "admin") router.replace(getDashboardRoute(user.role));
  }, [isLoading, isAuthenticated, user, router]);

  const analytics = useTicketAnalytics();
  const users = useUsers({ page: 1, limit: 1 });
  const teams = useTeams({ page: 1, limit: 1 });
  const rounds = useJudgingRounds({ enabled: isAdmin });
  const audit = useAuditLogs({ page: 1, limit: 6 });
  // Live standings for the active round (polls, so each judge's lock shows up within seconds)
  const liveRoundId = rounds.data?.find((r) => r.status === "active")?.id ?? "";
  const standings = useAdminLeaderboard(liveRoundId);

  if (isLoading || !isAdmin) {
    return <SkeletonCard />;
  }

  const a = analytics.data;
  const rate = a ? attendancePercent(a.attendance_rate) : 0;
  const remaining = a ? Math.max(a.total_issued - a.checked_in_count, 0) : 0;

  const roundList = rounds.data || [];
  const activeRound = roundList.find((r) => r.status === "active");
  const draftCount = roundList.filter((r) => r.status === "draft").length;

  const arrivals = a?.hourly_arrivals || [];
  const peak = Math.max(...arrivals.map((h) => h.count), 1);
  const needsAttention = (a?.team_completeness || [])
    .filter((t) => t.status !== "complete")
    .sort((x, y) => x.checked_in_members / (x.total_members || 1) - y.checked_in_members / (y.total_members || 1))
    .slice(0, 5);

  const dateLabel = new Date().toLocaleDateString([], { weekday: "long", month: "long", day: "numeric" });

  return (
    <div className="font-sans space-y-6">
      {/* Header + quick actions */}
      <header className="flex flex-col sm:flex-row sm:items-end justify-between gap-3">
        <div>
          <p className="text-xs font-medium text-[var(--ink-secondary)]">{dateLabel}</p>
          <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-[var(--ink-primary)]">
            Event control
          </h1>
        </div>
        <div className="flex flex-wrap gap-2">
          {[
            { label: "Broadcast", href: "/dashboard/admin/notifications", icon: Bell },
            { label: "Check-in", href: "/dashboard/tickets", icon: QrCode },
            { label: "Judging", href: "/dashboard/judging", icon: Award },
          ].map((q) => (
            <Link
              key={q.label}
              href={q.href}
              className="inline-flex h-9 items-center gap-1.5 rounded-[10px] border border-[var(--card-line)] bg-[var(--card-solid)] px-3 text-xs font-semibold text-[var(--ink-primary)] transition-[background-color,transform] duration-150 hover:bg-[var(--card-solid-subtle)] active:scale-[0.97]"
            >
              <q.icon className="h-3.5 w-3.5 text-[var(--ink-secondary)]" />
              {q.label}
            </Link>
          ))}
        </div>
      </header>

      {/* Hero: attendance is the live number on event day */}
      <section className="rounded-[20px] border border-[var(--card-line)] bg-[var(--card-solid)] p-5 sm:p-6">
        <div className="flex flex-col lg:flex-row lg:items-end justify-between gap-6">
          <div>
            <div className="text-xs font-semibold uppercase tracking-wider text-[var(--ink-secondary)]">
              Checked in
            </div>
            {analytics.isLoading ? (
              <Skeleton className="h-14 w-48 mt-2" />
            ) : (
              <div className="mt-1 flex items-baseline gap-3">
                <span className="text-5xl sm:text-6xl font-bold tabular-nums tracking-tight text-[var(--ink-primary)]">
                  {a?.checked_in_count ?? 0}
                </span>
                <span className="text-lg text-[var(--ink-secondary)] tabular-nums">
                  / {a?.total_issued ?? 0} passes
                </span>
              </div>
            )}
          </div>

          <dl className="grid grid-cols-3 gap-6 text-sm">
            <div>
              <dt className="text-xs text-[var(--ink-secondary)]">Attendance</dt>
              <dd className="text-xl font-bold tabular-nums text-[var(--ink-primary)]">{rate}%</dd>
            </div>
            <div>
              <dt className="text-xs text-[var(--ink-secondary)]">Yet to arrive</dt>
              <dd className="text-xl font-bold tabular-nums text-[var(--ink-primary)]">{remaining}</dd>
            </div>
            <div>
              <dt className="text-xs text-[var(--ink-secondary)]">Teams incomplete</dt>
              <dd className="text-xl font-bold tabular-nums text-[var(--ink-primary)]">
                {(a?.team_completeness || []).filter((t) => t.status !== "complete").length}
              </dd>
            </div>
          </dl>
        </div>

        <div
          className="mt-5 h-2 w-full overflow-hidden rounded-full bg-[var(--card-solid-subtle)]"
          role="progressbar"
          aria-valuenow={rate}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-label="Attendance"
        >
          <div
            className="h-full rounded-full bg-[var(--color-blue)] transition-[width] duration-500 ease-[cubic-bezier(0.23,1,0.32,1)]"
            style={{ width: `${Math.min(rate, 100)}%` }}
          />
        </div>
      </section>

      {/* Totals */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <Stat label="Users" value={users.data?.total_count ?? "—"} note="Manage accounts" href="/dashboard/users" icon={Users} />
        <Stat label="Teams" value={teams.data?.total_count ?? "—"} note="View rosters" href="/dashboard/teams" icon={Layers} />
        <Stat
          label="Judging rounds"
          value={roundList.length || "—"}
          note={activeRound ? `Live: ${activeRound.name}` : draftCount ? `${draftCount} in draft` : "None active"}
          href="/dashboard/judging"
          icon={Award}
        />
        <Stat label="Audit events" value={audit.data?.total_count ?? "—"} note="Full trail" href="/dashboard/audit" icon={FileText} />
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6 items-start">
        <div className="xl:col-span-2 space-y-6 min-w-0">
          {/* Arrivals */}
          <Panel title="Arrivals by hour" action={{ label: "Check-in desk", href: "/dashboard/tickets" }}>
            {arrivals.length === 0 ? (
              <p className="py-6 text-sm text-[var(--ink-secondary)]">No check-ins yet. Arrivals will chart here as people scan in.</p>
            ) : (
              <div className="flex items-end gap-1.5 h-40" role="img" aria-label="Hourly arrivals bar chart">
                {arrivals.map((h) => (
                  <div key={h.hour} className="flex-1 min-w-0 flex flex-col items-center justify-end h-full gap-1.5 group">
                    <span className="text-[10px] tabular-nums text-[var(--ink-secondary)] opacity-0 group-hover:opacity-100 transition-opacity">
                      {h.count}
                    </span>
                    <div
                      className="w-full rounded-t-[6px] bg-[var(--color-blue)]/80 group-hover:bg-[var(--color-blue)] transition-colors"
                      style={{ height: `${Math.max((h.count / peak) * 100, 4)}%` }}
                    />
                    <span className="text-[10px] text-[var(--ink-tertiary)] truncate max-w-full">
                      {h.hour.length > 5 ? h.hour.slice(11, 16) || h.hour : h.hour}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </Panel>

          {/* Recent activity */}
          <Panel title="Recent activity" action={{ label: "All events", href: "/dashboard/audit" }}>
            {audit.isLoading ? (
              <div className="space-y-3">
                {[0, 1, 2, 3].map((i) => (
                  <Skeleton key={i} className="h-9 w-full" />
                ))}
              </div>
            ) : (audit.data?.data || []).length === 0 ? (
              <p className="py-4 text-sm text-[var(--ink-secondary)]">Nothing has happened yet.</p>
            ) : (
              <ul className="divide-y divide-[var(--card-line)] -my-1">
                {(audit.data?.data || []).map((e) => (
                  <li key={e.id} className="flex items-start justify-between gap-4 py-2.5">
                    <div className="min-w-0">
                      <p className="text-sm text-[var(--ink-primary)] truncate">{e.description}</p>
                      <p className="text-xs text-[var(--ink-secondary)] truncate">
                        {e.actor_email || "System"}
                      </p>
                    </div>
                    <span className="shrink-0 text-xs tabular-nums text-[var(--ink-tertiary)]">
                      {timeAgo(e.created_at)}
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </Panel>
        </div>

        <div className="space-y-6 min-w-0">
          {/* Judging now */}
          <Panel title="Judging" action={{ label: "Open", href: "/dashboard/judging" }}>
            {rounds.isLoading ? (
              <Skeleton className="h-20 w-full" />
            ) : activeRound ? (
              <div className="space-y-3">
                <div className="flex items-center gap-2">
                  <Badge variant="blue" dot size="sm">Live</Badge>
                  <span className="text-sm font-semibold text-[var(--ink-primary)] truncate">{activeRound.name}</span>
                </div>
                <dl className="grid grid-cols-3 gap-3 text-xs">
                  <div>
                    <dt className="text-[var(--ink-secondary)]">Panels</dt>
                    <dd className="text-lg font-bold tabular-nums text-[var(--ink-primary)]">{activeRound.panel_count ?? 0}</dd>
                  </div>
                  <div>
                    <dt className="text-[var(--ink-secondary)]">Slots</dt>
                    <dd className="text-lg font-bold tabular-nums text-[var(--ink-primary)]">{activeRound.slot_count ?? 0}</dd>
                  </div>
                  <div>
                    <dt className="text-[var(--ink-secondary)]">Slot length</dt>
                    <dd className="text-lg font-bold tabular-nums text-[var(--ink-primary)]">{activeRound.slot_duration_mins}m</dd>
                  </div>
                </dl>

                <div className="border-t border-[var(--card-line)] pt-3">
                  <div className="flex items-center justify-between text-xs text-[var(--ink-secondary)] mb-1.5">
                    <span>Standings</span>
                    {standings.data && (
                      <span className="tabular-nums">
                        {standings.data.graded_teams}/{standings.data.total_teams} fully scored
                      </span>
                    )}
                  </div>
                  {standings.isLoading ? (
                    <Skeleton className="h-16 w-full" />
                  ) : (
                    (() => {
                      const top = (standings.data?.leaderboard ?? [])
                        .filter((e) => e.judges_evaluated_count > 0)
                        .slice(0, 5);
                      return top.length === 0 ? (
                        <p className="text-xs text-[var(--ink-tertiary)]">No scorecards locked yet.</p>
                      ) : (
                        <ol className="space-y-1">
                          {top.map((e) => (
                            <li key={e.team_id} className="flex items-center gap-2 text-xs">
                              <span className="w-4 font-mono font-bold tabular-nums text-[var(--ink-tertiary)]">{e.rank}</span>
                              <span className="flex-1 truncate font-medium text-[var(--ink-primary)]">{e.team_name}</span>
                              <span className="font-mono tabular-nums text-[var(--ink-primary)]">
                                {Math.round(e.total_weighted_score * 100) / 100}
                                <span className="text-[var(--ink-tertiary)]"> / {Math.round(e.max_possible_score * 100) / 100}</span>
                              </span>
                              {!e.all_panel_judges_locked && (
                                <span
                                  className="text-[10px] text-[var(--ink-tertiary)] tabular-nums"
                                  title="Provisional: not every judge has locked yet"
                                >
                                  {e.judges_evaluated_count}/{e.total_judges_on_panel}
                                </span>
                              )}
                            </li>
                          ))}
                        </ol>
                      );
                    })()
                  )}
                </div>
              </div>
            ) : (
              <div className="space-y-3">
                <p className="text-sm text-[var(--ink-secondary)]">
                  {draftCount ? `${draftCount} draft round${draftCount === 1 ? "" : "s"} ready to activate.` : "No rounds created yet."}
                </p>
                <Link
                  href="/dashboard/judging"
                  className="inline-flex items-center gap-1 text-xs font-semibold text-[var(--color-blue)] hover:underline"
                >
                  {draftCount ? "Activate a round" : "Create a round"}
                  <ArrowUpRight className="h-3 w-3" />
                </Link>
              </div>
            )}
          </Panel>

          {/* Needs attention */}
          <Panel title="Teams to chase" action={{ label: "All teams", href: "/dashboard/teams" }}>
            {needsAttention.length === 0 ? (
              <p className="text-sm text-[var(--ink-secondary)]">
                {a ? "Every team is fully checked in." : "No data yet."}
              </p>
            ) : (
              <ul className="space-y-3">
                {needsAttention.map((t) => {
                  const pct = t.total_members ? (t.checked_in_members / t.total_members) * 100 : 0;
                  return (
                    <li key={t.team_id}>
                      <div className="flex items-baseline justify-between gap-3 text-sm">
                        <span className="truncate font-medium text-[var(--ink-primary)]">{t.team_name}</span>
                        <span className="shrink-0 text-xs tabular-nums text-[var(--ink-secondary)]">
                          {t.checked_in_members}/{t.total_members}
                        </span>
                      </div>
                      <div className="mt-1.5 h-1.5 rounded-full bg-[var(--card-solid-subtle)] overflow-hidden">
                        <div className="h-full rounded-full bg-[var(--color-coral)]" style={{ width: `${pct}%` }} />
                      </div>
                    </li>
                  );
                })}
              </ul>
            )}
          </Panel>
        </div>
      </div>
    </div>
  );
}
