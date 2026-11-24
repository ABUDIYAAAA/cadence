"use client";

import React, { useEffect } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  ArrowRight,
  Calendar,
  ChevronRight,
  FileText,
  Users,
} from "lucide-react";
import { useAuth } from "@/providers/auth-provider";
import { useTenant } from "@/providers/tenant-provider";
import { getDashboardRoute } from "@/lib/auth-routes";
import { usePlatformUsers } from "@/hooks/use-users";
import { useEvents } from "@/hooks/use-events";
import { useAuditLogs } from "@/hooks/use-audit";
import { Skeleton, SkeletonCard } from "@/components/ui/skeleton";

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
        <h2 className="text-sm font-semibold tracking-tight text-[var(--ink-primary)]">
          {title}
        </h2>
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
        <span className="text-xs font-medium text-[var(--ink-secondary)]">
          {label}
        </span>
        <Icon className="h-4 w-4 transition-colors group-hover:text-[var(--color-blue)]" />
      </div>
      <div className="mt-2 text-2xl font-bold tabular-nums tracking-tight text-[var(--ink-primary)]">
        {value}
      </div>
      {note && (
        <div className="mt-0.5 text-[11px] text-[var(--ink-secondary)] truncate">
          {note}
        </div>
      )}
    </Link>
  );
}

export default function DevDashboardPage() {
  const router = useRouter();
  const { user, effectiveRole, isDev, isAuthenticated, isLoading } = useAuth();
  const { selectEvent } = useTenant();

  useEffect(() => {
    if (isLoading) return;
    if (!isAuthenticated || !user) router.replace("/login");
    else if (!isDev) router.replace(getDashboardRoute(effectiveRole ?? user.role));
  }, [isLoading, isAuthenticated, user, isDev, effectiveRole, router]);

  const users = usePlatformUsers({ page: 1, limit: 100 }, { enabled: isDev });
  const activeUsers = usePlatformUsers(
    { page: 1, limit: 1, is_active: true },
    { enabled: isDev }
  );
  const devUsers = usePlatformUsers(
    { page: 1, limit: 1, role: "dev" },
    { enabled: isDev }
  );
  const eventsQuery = useEvents({ enabled: isDev });
  const audit = useAuditLogs({ page: 1, limit: 6 });

  if (isLoading || !isDev) {
    return <SkeletonCard />;
  }

  const totalUsers = users.data?.total_count ?? 0;
  const activeCount = activeUsers.data?.total_count ?? 0;
  const devCount = devUsers.data?.total_count ?? 0;
  const eventList = eventsQuery.data ?? [];
  const activeRate = totalUsers > 0 ? Math.round((activeCount / totalUsers) * 100) : 0;

  const dateLabel = new Date().toLocaleDateString([], {
    weekday: "long",
    month: "long",
    day: "numeric",
  });

  return (
    <div className="font-sans space-y-6">
      {/* Header + quick actions */}
      <header className="flex flex-col sm:flex-row sm:items-end justify-between gap-3">
        <div>
          <p className="text-xs font-medium text-[var(--ink-secondary)]">
            {dateLabel}
          </p>
          <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-[var(--ink-primary)]">
            Platform control
          </h1>
        </div>
        <div className="flex flex-wrap gap-2">
          {[
            { label: "Global users", href: "/dashboard/users", icon: Users },
            { label: "Events", href: "/events", icon: Calendar },
            { label: "Audit logs", href: "/dashboard/audit", icon: FileText },
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

      {/* Hero: platform account overview */}
      <section className="rounded-[20px] border border-[var(--card-line)] bg-[var(--card-solid)] p-5 sm:p-6">
        <div className="flex flex-col lg:flex-row lg:items-end justify-between gap-6">
          <div>
            <div className="text-xs font-semibold uppercase tracking-wider text-[var(--ink-secondary)]">
              Global users
            </div>
            {users.isLoading ? (
              <Skeleton className="h-14 w-48 mt-2" />
            ) : (
              <div className="mt-1 flex items-baseline gap-3">
                <span className="text-5xl sm:text-6xl font-bold tabular-nums tracking-tight text-[var(--ink-primary)]">
                  {totalUsers}
                </span>
                <span className="text-lg text-[var(--ink-secondary)] tabular-nums">
                  accounts
                </span>
              </div>
            )}
          </div>

          <dl className="grid grid-cols-3 gap-6 text-sm">
            <div>
              <dt className="text-xs text-[var(--ink-secondary)]">Active</dt>
              <dd className="text-xl font-bold tabular-nums text-[var(--ink-primary)]">
                {activeUsers.isLoading ? "—" : activeCount}
              </dd>
            </div>
            <div>
              <dt className="text-xs text-[var(--ink-secondary)]">Developers</dt>
              <dd className="text-xl font-bold tabular-nums text-[var(--ink-primary)]">
                {devUsers.isLoading ? "—" : devCount}
              </dd>
            </div>
            <div>
              <dt className="text-xs text-[var(--ink-secondary)]">Events</dt>
              <dd className="text-xl font-bold tabular-nums text-[var(--ink-primary)]">
                {eventsQuery.isLoading ? "—" : eventList.length}
              </dd>
            </div>
          </dl>
        </div>

        <div
          className="mt-5 h-2 w-full overflow-hidden rounded-full bg-[var(--card-solid-subtle)]"
          role="progressbar"
          aria-valuenow={activeRate}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-label="Active accounts"
        >
          <div
            className="h-full rounded-full bg-[var(--color-blue)] transition-[width] duration-500 ease-[cubic-bezier(0.23,1,0.32,1)]"
            style={{ width: `${Math.min(activeRate, 100)}%` }}
          />
        </div>
      </section>

      {/* Totals */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <Stat
          label="Global users"
          value={users.isLoading ? "—" : totalUsers}
          note="Manage platform accounts"
          href="/dashboard/users"
          icon={Users}
        />
        <Stat
          label="Events"
          value={eventsQuery.isLoading ? "—" : eventList.length}
          note="Create & enter workspaces"
          href="/events"
          icon={Calendar}
        />
        <Stat
          label="Audit logs"
          value={audit.isLoading ? "—" : audit.data?.total_count ?? "—"}
          note="Platform security trail"
          href="/dashboard/audit"
          icon={FileText}
        />
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6 items-start">
        <div className="xl:col-span-2 space-y-6 min-w-0">
          {/* Recent activity */}
          <Panel
            title="Recent activity"
            action={{ label: "All logs", href: "/dashboard/audit" }}
          >
            {audit.isLoading ? (
              <div className="space-y-3">
                {[0, 1, 2, 3].map((i) => (
                  <Skeleton key={i} className="h-9 w-full" />
                ))}
              </div>
            ) : (audit.data?.data || []).length === 0 ? (
              <p className="py-4 text-sm text-[var(--ink-secondary)]">
                Nothing has happened yet.
              </p>
            ) : (
              <ul className="divide-y divide-[var(--card-line)] -my-1">
                {(audit.data?.data || []).map((e) => (
                  <li
                    key={e.id}
                    className="flex items-start justify-between gap-4 py-2.5"
                  >
                    <div className="min-w-0">
                      <p className="text-sm text-[var(--ink-primary)] truncate">
                        {e.description}
                      </p>
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
          {/* Events */}
          <Panel title="Events" action={{ label: "Manage", href: "/events" }}>
            {eventsQuery.isLoading ? (
              <div className="space-y-3">
                {[0, 1, 2].map((i) => (
                  <Skeleton key={i} className="h-10 w-full" />
                ))}
              </div>
            ) : eventList.length === 0 ? (
              <p className="py-4 text-sm text-[var(--ink-secondary)]">
                No events created yet.
              </p>
            ) : (
              <ul className="divide-y divide-[var(--card-line)] -my-1">
                {eventList.slice(0, 6).map((ev) => {
                  const memberCount = ev.member_count ?? 0;
                  return (
                    <li key={ev.id}>
                      <button
                        type="button"
                        onClick={() => {
                          selectEvent(ev.slug);
                          router.push("/dashboard/admin");
                        }}
                        className="group w-full flex items-center justify-between gap-3 py-2.5 text-left cursor-pointer"
                      >
                        <div className="flex items-center gap-3 min-w-0">
                          <div className="flex h-8 w-8 shrink-0 items-center justify-center overflow-hidden rounded-[6px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)]">
                            {ev.logo_url ? (
                              <img
                                src={ev.logo_url}
                                alt={`${ev.name} logo`}
                                className="h-full w-full object-cover"
                              />
                            ) : (
                              <Calendar className="h-3.5 w-3.5 text-[var(--ink-secondary)]" />
                            )}
                          </div>
                          <div className="min-w-0">
                            <p className="text-sm font-medium text-[var(--ink-primary)] truncate group-hover:text-[var(--color-blue)] transition-colors">
                              {ev.name}
                            </p>
                            <p className="text-xs text-[var(--ink-secondary)]">
                              {memberCount}{" "}
                              {memberCount === 1 ? "member" : "members"}
                            </p>
                          </div>
                        </div>
                        <ChevronRight className="h-4 w-4 shrink-0 text-[var(--ink-tertiary)] transition-transform group-hover:translate-x-0.5 group-hover:text-[var(--ink-primary)]" />
                      </button>
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
