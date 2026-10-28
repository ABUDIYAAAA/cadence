"use client";

import React, { useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  Users,
  LayoutDashboard,
  Menu,
  X,
  Layers,
  Award,
  Ticket,
  FileText,
  Bell,
  Activity,
  ExternalLink,
} from "lucide-react";
import { useAuth } from "@/providers/auth-provider";
import { getDashboardRoute } from "@/lib/auth-routes";
import { UserRole } from "@/types/auth";
import { cn } from "@/lib/utils";

interface NavItem {
  label: string;
  href: string;
  icon: React.ComponentType<{ className?: string }>;
  roles?: UserRole[];
  external?: boolean;
}

export function Sidebar() {
  const pathname = usePathname();
  const { user } = useAuth();
  const [isOpenMobile, setIsOpenMobile] = useState(false);

  const dashboardHref = user ? getDashboardRoute(user.role) : "/dashboard";

  const allNavItems: NavItem[] = [
    {
      label: "Overview",
      href: dashboardHref,
      icon: LayoutDashboard,
    },
    {
      label: "Users",
      href: "/dashboard/users",
      icon: Users,
      roles: ["admin"],
    },
    {
      label: "Teams",
      href: "/dashboard/teams",
      icon: Layers,
      roles: ["admin", "volunteer", "judge"],
    },
    {
      label: "Broadcast",
      href: "/dashboard/admin/notifications",
      icon: Bell,
      roles: ["admin"],
    },
    {
      label: "Judging",
      href: "/dashboard/judging",
      icon: Award,
      roles: ["admin", "judge", "volunteer", "participant"],
    },
    {
      label: "Tickets",
      href: "/dashboard/tickets",
      icon: Ticket,
      roles: ["admin", "volunteer", "participant"],
    },
    {
      label: "Audit Logs",
      href: "/dashboard/audit",
      icon: FileText,
      roles: ["admin"],
    },
    {
      label: "Grafana Metrics",
      href: "http://localhost:3001/d/cadence-overview/cadence-hackathon-operations-and-telemetry",
      icon: Activity,
      roles: ["admin"],
      external: true,
    },
  ];

  const visibleNavItems = allNavItems.filter((item) =>
    !item.roles || (user?.role && item.roles.includes(user.role))
  );

  const isActive = (href: string) => {
    if (href === "/dashboard" || href === dashboardHref) {
      return pathname === href;
    }
    return pathname.startsWith(href);
  };

  return (
    <>
      {/* Mobile Floating Toggle Button */}
      <div className="md:hidden fixed bottom-5 right-5 z-50">
        <button
          onClick={() => setIsOpenMobile(!isOpenMobile)}
          aria-label="Toggle Navigation Sidebar"
          className="h-12 w-12 rounded-full bg-[var(--color-dark)] text-[var(--ink-inverse)] shadow-xl flex items-center justify-center transition-transform active:scale-90 cursor-pointer"
        >
          {isOpenMobile ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
        </button>
      </div>

      {/* Mobile Drawer Backdrop & Drawer */}
      {isOpenMobile && (
        <div className="fixed inset-0 z-50 md:hidden">
          <div
            className="fixed inset-0 bg-black/40 backdrop-blur-sm transition-opacity"
            onClick={() => setIsOpenMobile(false)}
          />
          <aside className="fixed inset-y-0 left-0 z-50 w-72 bg-[var(--card-solid)] border-r border-[var(--card-line)] p-5 shadow-2xl flex flex-col justify-between font-sans">
            <div>
              <div className="flex items-center justify-between pb-4 border-b border-[var(--card-line)] mb-4">
                <span className="text-xs font-extrabold tracking-wider text-[var(--ink-primary)] uppercase">
                  Cadence Navigation
                </span>
                <button
                  onClick={() => setIsOpenMobile(false)}
                  className="h-7 w-7 rounded-full bg-[var(--card-solid-subtle)] text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] flex items-center justify-center cursor-pointer"
                >
                  ✕
                </button>
              </div>

              <nav className="space-y-1">
                {visibleNavItems.map((item) => {
                  const active = isActive(item.href);
                  const Icon = item.icon;

                  if (item.external) {
                    return (
                      <a
                        key={item.href}
                        href={item.href}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="flex items-center justify-between px-3.5 py-2.5 rounded-[12px] text-xs font-semibold text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] hover:bg-[var(--card-solid-subtle)] transition-all"
                      >
                        <div className="flex items-center gap-3">
                          <Icon className="h-4 w-4" />
                          <span>{item.label}</span>
                        </div>
                        <ExternalLink className="h-3 w-3 opacity-60" />
                      </a>
                    );
                  }

                  return (
                    <Link
                      key={item.href}
                      href={item.href}
                      onClick={() => setIsOpenMobile(false)}
                      className={cn(
                        "flex items-center justify-between px-3.5 py-2.5 rounded-[12px] text-xs font-semibold transition-all active:scale-95",
                        active
                          ? "bg-[var(--color-dark)] text-[var(--ink-inverse)] shadow-sm"
                          : "text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] hover:bg-[var(--card-solid-subtle)]"
                      )}
                    >
                      <div className="flex items-center gap-3">
                        <Icon className="h-4 w-4" />
                        <span>{item.label}</span>
                      </div>
                    </Link>
                  );
                })}
              </nav>
            </div>

            {user && (
              <div className="pt-4 border-t border-[var(--card-line)]">
                <div className="p-3 bg-[var(--card-solid-subtle)] rounded-[14px] border border-[var(--card-line)]">
                  <div className="text-xs font-bold text-[var(--ink-primary)] truncate">
                    {user.name}
                  </div>
                  <div className="text-[11px] text-[var(--ink-secondary)] truncate">
                    {user.role.toUpperCase()}
                  </div>
                </div>
              </div>
            )}
          </aside>
        </div>
      )}

      {/* Desktop Floating Vertical Dock (Exact preview2.html implementation) */}
      <aside className="hidden md:flex flex-col items-center w-[58px] rounded-full left-vertical-dock py-3 px-1.5 gap-2.5 sticky top-20 shrink-0 z-30 font-sans">
        {visibleNavItems.map((item) => {
          const active = isActive(item.href);
          const Icon = item.icon;

          if (item.external) {
            return (
              <a
                key={item.href}
                href={item.href}
                target="_blank"
                rel="noopener noreferrer"
                title={item.label}
                className="group relative flex items-center justify-center h-[42px] w-[42px] rounded-full text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] hover:bg-[var(--card-solid-subtle)] transition-all active:scale-90"
              >
                <Icon className="h-[18px] w-[18px]" />
                {/* Tooltip */}
                <span className="pointer-events-none absolute left-full ml-3 hidden group-hover:flex items-center px-2.5 py-1 rounded-[8px] bg-[var(--color-dark)] text-[var(--ink-inverse)] text-[11px] font-bold shadow-md whitespace-nowrap z-50 animate-in fade-in-0 duration-100">
                  {item.label}
                </span>
              </a>
            );
          }

          return (
            <Link
              key={item.href}
              href={item.href}
              title={item.label}
              className={cn(
                "group relative flex items-center justify-center h-[42px] w-[42px] rounded-full transition-all active:scale-90",
                active
                  ? "bg-[var(--card-solid)] text-[var(--ink-primary)] shadow-[var(--shadow-pill)]"
                  : "text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] hover:bg-[var(--card-solid-subtle)]"
              )}
            >
              <Icon className="h-[18px] w-[18px]" />
              {/* Tooltip */}
              <span className="pointer-events-none absolute left-full ml-3 hidden group-hover:flex items-center px-2.5 py-1 rounded-[8px] bg-[var(--color-dark)] text-[var(--ink-inverse)] text-[11px] font-bold shadow-md whitespace-nowrap z-50 animate-in fade-in-0 duration-100">
                {item.label}
              </span>
            </Link>
          );
        })}
      </aside>
    </>
  );
}
