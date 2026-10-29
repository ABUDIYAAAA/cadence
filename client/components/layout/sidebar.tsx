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
  ArrowUpRight,
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

// Fixed geometry so one indicator can glide between rows with a single transform.
const ITEM_H = 44;
const ITEM_GAP = 4;
const EASE_DRAWER = "ease-[cubic-bezier(0.32,0.72,0,1)]";

interface NavListProps {
  items: NavItem[];
  activeIndex: number;
  /** Labels always visible (mobile drawer). Otherwise revealed by rail hover/focus. */
  pinned?: boolean;
  onNavigate?: () => void;
}

function NavList({ items, activeIndex, pinned, onNavigate }: NavListProps) {
  const labelCls = cn(
    "whitespace-nowrap text-[13px] transition-opacity duration-200 motion-reduce:transition-none",
    pinned ? "opacity-100" : "opacity-0 group-hover/rail:opacity-100 group-has-[:focus-visible]/rail:opacity-100"
  );

  return (
    <nav className="relative flex flex-col" style={{ gap: ITEM_GAP }} aria-label="Primary">
      {/* Single gliding indicator */}
      <span
        aria-hidden="true"
        className={cn(
          "absolute left-0 top-0 w-full rounded-[14px] bg-[var(--color-dark)] shadow-[0_6px_16px_-6px_rgba(10,14,24,0.45)]",
          "transition-[transform,opacity] duration-300 motion-reduce:transition-none",
          EASE_DRAWER,
          activeIndex < 0 ? "opacity-0" : "opacity-100"
        )}
        style={{
          height: ITEM_H,
          transform: `translateY(${Math.max(activeIndex, 0) * (ITEM_H + ITEM_GAP)}px)`,
        }}
      />

      {items.map((item, i) => {
        const active = i === activeIndex;
        const Icon = item.icon;
        const base = cn(
          "relative z-10 flex items-center rounded-[14px] outline-none",
          "focus-visible:ring-2 focus-visible:ring-[var(--color-blue)] focus-visible:ring-offset-0",
          "transition-[color,background-color,transform] duration-150 active:scale-[0.97]",
          active
            ? "text-[var(--ink-inverse)]"
            : "text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] hover:bg-[var(--card-solid-subtle)]"
        );

        const inner = (
          <>
            <span className="grid place-items-center shrink-0" style={{ width: ITEM_H, height: ITEM_H }}>
              <Icon className="h-[18px] w-[18px]" />
            </span>
            <span className={cn(labelCls, active ? "font-semibold" : "font-medium")}>{item.label}</span>
            {item.external && (
              <ArrowUpRight className={cn("ml-auto mr-3 h-3.5 w-3.5 shrink-0", labelCls)} />
            )}
          </>
        );

        return item.external ? (
          <a
            key={item.href}
            href={item.href}
            target="_blank"
            rel="noopener noreferrer"
            className={base}
            style={{ height: ITEM_H }}
          >
            {inner}
          </a>
        ) : (
          <Link
            key={item.href}
            href={item.href}
            onClick={onNavigate}
            aria-current={active ? "page" : undefined}
            className={base}
            style={{ height: ITEM_H }}
          >
            {inner}
          </Link>
        );
      })}
    </nav>
  );
}

function UserChip({ name, role, pinned }: { name: string; role: string; pinned?: boolean }) {
  const initials = name
    .split(" ")
    .map((p) => p[0])
    .filter(Boolean)
    .slice(0, 2)
    .join("")
    .toUpperCase();

  return (
    <div className="flex items-center rounded-[14px]" style={{ height: ITEM_H }}>
      <span
        className="grid place-items-center shrink-0"
        style={{ width: ITEM_H, height: ITEM_H }}
      >
        <span className="grid h-8 w-8 place-items-center rounded-full bg-[var(--color-blue-dim)] text-[11px] font-semibold text-[var(--color-blue)] ring-1 ring-[var(--color-blue-border)]">
          {initials || "?"}
        </span>
      </span>
      <span
        className={cn(
          "min-w-0 pr-3 leading-tight transition-opacity duration-200 motion-reduce:transition-none",
          pinned ? "opacity-100" : "opacity-0 group-hover/rail:opacity-100 group-has-[:focus-visible]/rail:opacity-100"
        )}
      >
        <span className="block truncate text-[13px] font-semibold text-[var(--ink-primary)]">{name}</span>
        <span className="block truncate text-[11px] capitalize text-[var(--ink-secondary)]">{role}</span>
      </span>
    </div>
  );
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

  useEffect(() => {
    if (!isOpenMobile) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setIsOpenMobile(false);
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [isOpenMobile]);

  const dashboardHref = user ? getDashboardRoute(user.role) : "/dashboard";

  const allNavItems: NavItem[] = [
    { label: "Overview", href: dashboardHref, icon: LayoutDashboard },
    { label: "Users", href: "/dashboard/users", icon: Users, roles: ["admin"] },
    { label: "Teams", href: "/dashboard/teams", icon: Layers, roles: ["admin", "volunteer", "judge"] },
    { label: "Broadcasts", href: "/dashboard/admin/notifications", icon: Bell, roles: ["admin"] },
    {
      label: "Judging",
      href: "/dashboard/judging",
      icon: Award,
      roles: ["admin", "judge", "volunteer", "participant"],
    },
    { label: "Tickets", href: "/dashboard/tickets", icon: Ticket, roles: ["admin", "volunteer", "participant"] },
    { label: "Audit log", href: "/dashboard/audit", icon: FileText, roles: ["admin"] },
    {
      label: "Metrics",
      href: "http://localhost:3001/d/cadence-overview/cadence-hackathon-operations-and-telemetry",
      icon: Activity,
      roles: ["admin"],
      external: true,
    },
  ];

  const visibleNavItems = allNavItems.filter(
    (item) => !item.roles || (user?.role && item.roles.includes(user.role))
  );

  const isActive = (item: NavItem) => {
    if (item.external) return false;
    if (item.href === "/dashboard" || item.href === dashboardHref) return pathname === item.href;
    return pathname.startsWith(item.href);
  };
  const activeIndex = visibleNavItems.findIndex(isActive);

  return (
    <>
      {/* Mobile Floating Toggle Button */}
      <div className="md:hidden fixed bottom-5 right-5 z-50">
        <button
          onClick={() => setIsOpenMobile(!isOpenMobile)}
          aria-label="Toggle navigation menu"
          aria-expanded={isOpenMobile}
          className="h-12 w-12 rounded-full bg-[var(--color-dark)] text-[var(--ink-inverse)] shadow-lg flex items-center justify-center transition-transform duration-150 active:scale-[0.95] cursor-pointer"
        >
          {isOpenMobile ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
        </button>
      </div>

      {/* Mobile Drawer */}
      <div
        className={cn(
          "fixed inset-0 z-40 md:hidden",
          isOpenMobile ? "pointer-events-auto visible" : "pointer-events-none invisible delay-300"
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
            "fixed inset-y-0 left-0 w-64 bg-[var(--card-solid)] p-3 pt-5 shadow-2xl flex flex-col justify-between font-sans",
            "transition-transform duration-300 motion-reduce:transition-none",
            EASE_DRAWER,
            isOpenMobile ? "translate-x-0" : "-translate-x-full"
          )}
        >
          <NavList
            items={visibleNavItems}
            activeIndex={activeIndex}
            pinned
            onNavigate={() => setIsOpenMobile(false)}
          />
          {user && (
            <div className="pt-3 border-t border-[var(--card-line)]">
              <UserChip name={user.name} role={user.role} pinned />
            </div>
          )}
        </aside>
      </div>

      {/* Desktop rail: reserves 72px, expands over content on hover/focus (no layout shift) */}
      <div className="hidden md:block relative shrink-0 w-[72px] sticky top-14 h-[calc(100vh-3.5rem)] z-30">
        <aside
          className={cn(
            "group/rail absolute inset-y-0 left-0 flex flex-col justify-between overflow-hidden font-sans",
            "w-[72px] hover:w-[232px] has-[:focus-visible]:w-[232px] p-3",
            "bg-[var(--card-solid)] border-r border-[var(--card-line)]",
            "transition-[width,box-shadow] duration-300 motion-reduce:transition-none",
            EASE_DRAWER,
            "hover:shadow-[12px_0_32px_-16px_rgba(10,14,24,0.25)] has-[:focus-visible]:shadow-[12px_0_32px_-16px_rgba(10,14,24,0.25)]"
          )}
        >
          <NavList items={visibleNavItems} activeIndex={activeIndex} />
          {user && (
            <div className="pt-3 border-t border-[var(--card-line)]">
              <UserChip name={user.name} role={user.role} />
            </div>
          )}
        </aside>
      </div>
    </>
  );
}
