"use client";

import React, { useState, useEffect } from "react";
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

  // Listen for toggle events from the Navbar mobile button
  useEffect(() => {
    const handleToggle = () => setIsOpenMobile((prev) => !prev);
    window.addEventListener("cadence:toggle-sidebar", handleToggle);
    return () => window.removeEventListener("cadence:toggle-sidebar", handleToggle);
  }, []);

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
      label: "Broadcasts",
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
      label: "Audit log",
      href: "/dashboard/audit",
      icon: FileText,
      roles: ["admin"],
    },
    {
      label: "Metrics",
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
          aria-label="Toggle navigation menu"
          className="h-11 w-11 rounded-[10px] bg-[var(--color-dark)] text-[var(--ink-inverse)] shadow-md flex items-center justify-center transition-transform active:scale-95 cursor-pointer"
        >
          {isOpenMobile ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
        </button>
      </div>

      {/* Mobile Drawer */}
      <div
        className={cn(
          "fixed inset-0 z-50 md:hidden transition-all duration-200",
          isOpenMobile ? "pointer-events-auto visible" : "pointer-events-none invisible"
        )}
      >
        <div
          className={cn(
            "fixed inset-0 bg-black/40 backdrop-blur-xs transition-opacity duration-200 ease-out",
            isOpenMobile ? "opacity-100" : "opacity-0"
          )}
          onClick={() => setIsOpenMobile(false)}
          aria-hidden="true"
        />
        <aside
          className={cn(
            "fixed inset-y-0 left-0 z-50 w-64 bg-[var(--card-solid)] border-r border-[var(--card-line)] p-4 shadow-xl flex flex-col justify-between font-sans transition-transform duration-200 ease-out transform",
            isOpenMobile ? "translate-x-0" : "-translate-x-full"
          )}
        >
          <div>
            <div className="flex items-center justify-between pb-3 border-b border-[var(--card-line)] mb-3">
              <span className="text-xs font-semibold text-[var(--ink-secondary)]">
                Navigation
              </span>
              <button
                onClick={() => setIsOpenMobile(false)}
                className="h-7 w-7 rounded-[6px] bg-[var(--card-solid-subtle)] text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] flex items-center justify-center cursor-pointer"
                aria-label="Close navigation"
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
                      className="flex items-center justify-between px-3 py-2 rounded-[8px] text-xs font-medium text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] hover:bg-[var(--card-solid-subtle)] transition-colors"
                    >
                      <div className="flex items-center gap-2.5">
                        <Icon className="h-4 w-4 text-[var(--ink-secondary)]" />
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
                      "flex items-center justify-between px-3 py-2 rounded-[8px] text-xs transition-colors duration-150 active:scale-[0.98]",
                      active
                        ? "bg-[var(--card-solid-subtle)] text-[var(--ink-primary)] font-semibold border-l-2 border-[var(--color-blue)]"
                        : "text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] hover:bg-[var(--card-solid-subtle)] font-medium"
                    )}
                  >
                    <div className="flex items-center gap-2.5">
                      <Icon className={cn("h-4 w-4", active ? "text-[var(--color-blue)]" : "text-[var(--ink-secondary)]")} />
                      <span>{item.label}</span>
                    </div>
                  </Link>
                );
              })}
            </nav>
          </div>

          {user && (
            <div className="pt-3 border-t border-[var(--card-line)]">
              <div className="p-2.5 bg-[var(--card-solid-subtle)] rounded-[8px] border border-[var(--card-line)]">
                <div className="text-xs font-semibold text-[var(--ink-primary)] truncate">
                  {user.name}
                </div>
                <div className="text-[11px] text-[var(--ink-secondary)] truncate capitalize">
                  {user.role}
                </div>
              </div>
            </div>
          )}
        </aside>
      </div>

      {/* Desktop Navigation Sidebar (Anchored full-height sidebar column) */}
      <aside className="hidden md:flex flex-col justify-between w-56 shrink-0 border-r border-[var(--card-line)] bg-[var(--card-solid)] p-3 sticky top-14 h-[calc(100vh-3.5rem)] overflow-y-auto font-sans">
        <nav className="space-y-0.5">
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
                  className="flex items-center justify-between px-2.5 py-2 rounded-[6px] text-xs font-medium text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] hover:bg-[var(--card-solid-subtle)] transition-colors"
                >
                  <div className="flex items-center gap-2.5">
                    <Icon className="h-4 w-4 text-[var(--ink-secondary)]" />
                    <span>{item.label}</span>
                  </div>
                  <ExternalLink className="h-3 w-3 opacity-50" />
                </a>
              );
            }

            return (
              <Link
                key={item.href}
                href={item.href}
                className={cn(
                  "flex items-center gap-2.5 px-2.5 py-2 rounded-[6px] text-xs transition-colors duration-150 active:scale-[0.98]",
                  active
                    ? "bg-[var(--card-solid-subtle)] text-[var(--ink-primary)] font-semibold border-l-2 border-[var(--color-blue)]"
                    : "text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] hover:bg-[var(--card-solid-subtle)] font-medium"
                )}
              >
                <Icon className={cn("h-4 w-4 shrink-0", active ? "text-[var(--color-blue)]" : "text-[var(--ink-secondary)]")} />
                <span className="truncate">{item.label}</span>
              </Link>
            );
          })}
        </nav>

        {user && (
          <div className="pt-2 mt-2 border-t border-[var(--card-line)] px-1 pb-1">
            <div className="text-[11px] font-medium text-[var(--ink-primary)] truncate">
              {user.name}
            </div>
            <div className="text-[10px] text-[var(--ink-secondary)] truncate capitalize">
              {user.role}
            </div>
          </div>
        )}
      </aside>
    </>
  );
}
