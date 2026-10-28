"use client";

import React, { useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  Users,
  LayoutDashboard,
  Menu,
  X,
  Shield,
  Layers,
  Award,
  Ticket,
  FileText,
} from "lucide-react";
import { useAuth } from "@/providers/auth-provider";
import { getDashboardRoute } from "@/lib/auth-routes";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface NavItem {
  label: string;
  href: string;
  icon: React.ComponentType<{ className?: string }>;
  badge?: string;
  adminOnly?: boolean;
}

export function Sidebar() {
  const pathname = usePathname();
  const { user, isAdmin } = useAuth();
  const [isOpenMobile, setIsOpenMobile] = useState(false);

  const dashboardHref = user ? getDashboardRoute(user.role) : "/dashboard";

  const adminNavItems: NavItem[] = [
    {
      label: "Users",
      href: "/dashboard/users",
      icon: Users,
      adminOnly: true,
    },
    {
      label: "Teams",
      href: "/dashboard/teams",
      icon: Layers,
      badge: "Beta",
      adminOnly: true,
    },
    {
      label: "Judging",
      href: "/dashboard/judging",
      icon: Award,
      adminOnly: true,
    },
    {
      label: "Tickets",
      href: "/dashboard/tickets",
      icon: Ticket,
      adminOnly: true,
    },
    {
      label: "Audit Logs",
      href: "/dashboard/audit",
      icon: FileText,
      adminOnly: true,
    },
  ];

  const isActive = (href: string) => {
    if (href === "/dashboard" || href === dashboardHref) {
      return pathname === href;
    }
    return pathname.startsWith(href);
  };

  const navContent = (
    <div className="flex flex-col h-full justify-between font-mono">
      <div className="space-y-6">
        {/* Main Section */}
        <div>
          <div className="px-3 mb-2 text-[10px] uppercase tracking-widest text-neutral-500 font-semibold">
            General
          </div>
          <nav className="space-y-1">
            <Link
              href={dashboardHref}
              onClick={() => setIsOpenMobile(false)}
              className={cn(
                "flex items-center justify-between px-3 py-2 text-xs font-semibold tracking-wider transition-colors",
                isActive(dashboardHref)
                  ? "bg-black text-white"
                  : "text-neutral-700 hover:bg-neutral-100 hover:text-black"
              )}
            >
              <div className="flex items-center gap-2.5">
                <LayoutDashboard className="h-4 w-4 shrink-0" />
                <span>Overview</span>
              </div>
              {isActive(dashboardHref) && (
                <span className="text-[10px] uppercase font-bold tracking-widest">
                  Active
                </span>
              )}
            </Link>
          </nav>
        </div>

        {/* Admin Navigation */}
        {isAdmin && (
          <div>
            <div className="flex items-center justify-between px-3 mb-2">
              <span className="text-[10px] uppercase tracking-widest text-neutral-500 font-semibold">
                Admin Operations
              </span>
              <Shield className="h-3 w-3 text-neutral-400" />
            </div>
            <nav className="space-y-1">
              {adminNavItems.map((item) => {
                const active = isActive(item.href);
                const Icon = item.icon;
                return (
                  <Link
                    key={item.href}
                    href={item.href}
                    onClick={() => setIsOpenMobile(false)}
                    className={cn(
                      "flex items-center justify-between px-3 py-2 text-xs font-semibold tracking-wider transition-colors border-l-2",
                      active
                        ? "bg-black text-white border-black"
                        : "text-neutral-700 border-transparent hover:bg-neutral-100 hover:text-black"
                    )}
                  >
                    <div className="flex items-center gap-2.5">
                      <Icon className="h-4 w-4 shrink-0" />
                      <span>{item.label}</span>
                    </div>
                    {item.badge && !active && (
                      <span className="text-[9px] uppercase px-1.5 py-0.2 bg-neutral-100 text-neutral-600 border border-neutral-300">
                        {item.badge}
                      </span>
                    )}
                  </Link>
                );
              })}
            </nav>
          </div>
        )}
      </div>

      {/* User Info footer in sidebar */}
      {user && (
        <div className="pt-4 border-t border-neutral-200 mt-6">
          <div className="p-3 bg-neutral-50 border border-neutral-200 text-xs space-y-1">
            <div className="flex items-center justify-between">
              <span className="font-bold text-black truncate max-w-[140px]">
                {user.name}
              </span>
              <Badge variant="solid" size="sm">
                {user.role}
              </Badge>
            </div>
            <div className="text-[11px] text-neutral-500 truncate">
              {user.email}
            </div>
          </div>
        </div>
      )}
    </div>
  );

  return (
    <>
      {/* Mobile Toggle Button */}
      <div className="md:hidden fixed bottom-4 right-4 z-50">
        <Button
          variant="primary"
          size="icon"
          onClick={() => setIsOpenMobile(!isOpenMobile)}
          aria-label="Toggle Navigation Sidebar"
          className="shadow-lg h-11 w-11 border border-black"
        >
          {isOpenMobile ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
        </Button>
      </div>

      {/* Mobile Backdrop & Drawer */}
      {isOpenMobile && (
        <div className="fixed inset-0 z-40 md:hidden">
          <div
            className="fixed inset-0 bg-black/60 backdrop-blur-xs"
            onClick={() => setIsOpenMobile(false)}
          />
          <aside className="fixed inset-y-0 left-0 z-50 w-64 bg-white border-r border-black p-4 shadow-xl">
            <div className="flex items-center justify-between pb-3 border-b border-neutral-200 mb-4">
              <span className="text-xs font-bold uppercase tracking-widest text-black">
                CADENCE MENU
              </span>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setIsOpenMobile(false)}
                className="h-6 w-6 p-0 text-sm"
              >
                ✕
              </Button>
            </div>
            {navContent}
          </aside>
        </div>
      )}

      {/* Desktop Persistent Sidebar */}
      <aside className="hidden md:flex flex-col w-60 shrink-0 border-r border-black bg-white p-4 sticky top-14 h-[calc(100vh-3.5rem)] overflow-y-auto">
        {navContent}
      </aside>
    </>
  );
}
