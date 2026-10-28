"use client";

import React, { useState, useRef, useEffect, useMemo } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { User, LogOut, LayoutDashboard, Bell, Sun, Moon } from "lucide-react";
import { NotificationBell } from "@/components/notifications/notification-bell";
import { useAuth } from "@/providers/auth-provider";
import { useToast } from "@/providers/toast-provider";
import { useNotificationsContext } from "@/providers/notification-provider";
import { getDashboardRoute } from "@/lib/auth-routes";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export function Navbar() {
  const pathname = usePathname();
  const { user, isAuthenticated, logout } = useAuth();
  const { isSupported, isSubscribed, enablePush, disablePush } =
    useNotificationsContext();
  const toast = useToast();
  const [isLoggingOut, setIsLoggingOut] = useState(false);
  const [dropdownOpen, setDropdownOpen] = useState(false);
  const [isDark, setIsDark] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  // Sync dark mode state with document class
  useEffect(() => {
    setIsDark(document.documentElement.classList.contains("dark"));
  }, []);

  const toggleTheme = () => {
    const nextDark = !isDark;
    setIsDark(nextDark);
    if (nextDark) {
      document.documentElement.classList.add("dark");
      document.documentElement.classList.remove("light");
      try {
        localStorage.setItem("theme", "dark");
      } catch {}
    } else {
      document.documentElement.classList.remove("dark");
      document.documentElement.classList.add("light");
      try {
        localStorage.setItem("theme", "light");
      } catch {}
    }
  };

  // Close dropdown on click outside or escape key
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(event.target as Node)
      ) {
        setDropdownOpen(false);
      }
    };

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setDropdownOpen(false);
      }
    };

    if (dropdownOpen) {
      document.addEventListener("mousedown", handleClickOutside);
      document.addEventListener("keydown", handleKeyDown);
    }

    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [dropdownOpen]);

  const handleLogout = async () => {
    setIsLoggingOut(true);
    try {
      await logout();
      setDropdownOpen(false);
      toast.info("You have been signed out.", "SESSION TERMINATED");
    } finally {
      setIsLoggingOut(false);
    }
  };

  const homeHref = isAuthenticated && user ? getDashboardRoute(user.role) : "/login";

  // Navigation Links based on role (Matching preview2.html header tabs)
  const navLinks = useMemo(() => {
    if (!isAuthenticated || !user) return [];
    const baseDashboard = getDashboardRoute(user.role);
    switch (user.role) {
      case "admin":
        return [
          { label: "Overview", href: baseDashboard },
          { label: "Users", href: "/dashboard/users" },
          { label: "Teams", href: "/dashboard/teams" },
          { label: "Judging", href: "/dashboard/judging" },
          { label: "Tickets", href: "/dashboard/tickets" },
          { label: "Audit", href: "/dashboard/audit" },
        ];
      case "judge":
        return [
          { label: "Overview", href: baseDashboard },
          { label: "Judging Panel", href: "/dashboard/judging" },
          { label: "Teams", href: "/dashboard/teams" },
        ];
      case "volunteer":
        return [
          { label: "Overview", href: baseDashboard },
          { label: "Gate Scanner", href: "/dashboard/tickets" },
          { label: "Teams", href: "/dashboard/teams" },
        ];
      case "participant":
        return [
          { label: "Overview", href: baseDashboard },
          { label: "My Team", href: "/dashboard/teams" },
          { label: "My Ticket", href: "/dashboard/tickets" },
          { label: "Queue Status", href: "/dashboard/judging" },
        ];
      default:
        return [{ label: "Dashboard", href: baseDashboard }];
    }
  }, [isAuthenticated, user]);

  const isLinkActive = (href: string) => {
    if (
      href === "/dashboard/admin" ||
      href === "/dashboard/judge" ||
      href === "/dashboard/volunteer" ||
      href === "/dashboard/participant" ||
      href === "/dashboard"
    ) {
      return pathname === href;
    }
    return pathname.startsWith(href);
  };

  // Compute initials
  const initials = user?.name
    ? user.name
        .split(" ")
        .map((n) => n[0])
        .join("")
        .slice(0, 2)
        .toUpperCase()
    : "CD";

  return (
    <header className="app-header font-sans transition-colors duration-200">
      <div className="mx-auto flex h-16 max-w-7xl items-center justify-between px-4 sm:px-6">
        {/* Brand Section */}
        <div className="flex items-center gap-6">
          <Link
            href={homeHref}
            className="flex items-center gap-2.5 text-decoration-none group select-none"
          >
            <span className="w-7 h-7 rounded-[8px] bg-[var(--color-dark)] text-[var(--ink-inverse)] font-extrabold text-xs flex items-center justify-center shadow-sm group-hover:scale-105 active:scale-95 transition-transform">
              C
            </span>
            <span className="text-lg font-extrabold tracking-tight text-[var(--ink-primary)]">
              Cadence
            </span>
          </Link>

          {/* Navigation Links (Matching preview2.html header tabs) */}
          {isAuthenticated && user && navLinks.length > 0 && (
            <nav className="hidden lg:flex items-center gap-1">
              {navLinks.map((link) => {
                const active = isLinkActive(link.href);
                return (
                  <Link
                    key={link.href}
                    href={link.href}
                    className={cn(
                      "rounded-full px-3.5 py-1 text-xs font-semibold transition-all active:scale-95 whitespace-nowrap",
                      active
                        ? "bg-[var(--color-dark)] text-[var(--ink-inverse)] shadow-[var(--shadow-dark)]"
                        : "text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] hover:bg-[var(--card-solid-subtle)]"
                    )}
                  >
                    {link.label}
                  </Link>
                );
              })}
            </nav>
          )}
        </div>

        {/* Right Section: Notification Bell, Theme Switcher & User Avatar */}
        <div className="flex items-center gap-2">
          {/* Theme Switcher Circle Button */}
          <button
            type="button"
            onClick={toggleTheme}
            aria-label="Toggle light/dark theme"
            className="circle-icon-btn !w-9 !h-9 text-[var(--ink-secondary)] hover:text-[var(--ink-primary)]"
          >
            {isDark ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
          </button>

          {isAuthenticated && user ? (
            <>
              {/* Notification Bell */}
              <NotificationBell />

              <div className="relative" ref={dropdownRef}>
                <button
                  type="button"
                  onClick={() => setDropdownOpen((prev) => !prev)}
                  aria-label="User menu"
                  aria-expanded={dropdownOpen}
                  className="flex items-center justify-center h-9 w-9 rounded-full bg-[var(--color-dark)] text-[var(--ink-inverse)] font-extrabold text-xs shadow-sm hover:scale-105 active:scale-95 transition-transform cursor-pointer select-none"
                >
                  {initials}
                </button>

                {/* Dropdown Menu (Floating Glass Card) */}
                {dropdownOpen && (
                  <div className="absolute right-0 mt-2 w-64 rounded-[20px] border border-[var(--card-border)] bg-[var(--card-solid)] shadow-[var(--shadow-container)] z-50 animate-in fade-in-0 zoom-in-95 duration-150 p-2 text-[var(--ink-primary)] font-sans">
                    {/* User Profile Header */}
                    <div className="p-3 bg-[var(--card-solid-subtle)] rounded-[14px] border border-[var(--card-line)] mb-1.5">
                      <div className="flex items-center justify-between mb-1">
                        <span className="text-[10px] uppercase font-bold text-[var(--ink-secondary)] tracking-wider">
                          Active Account
                        </span>
                        <Badge variant="solid" size="sm">
                          {user.role}
                        </Badge>
                      </div>
                      <p className="text-xs font-bold text-[var(--ink-primary)] truncate">
                        {user.name}
                      </p>
                      <p className="text-[11px] text-[var(--ink-secondary)] truncate font-mono">
                        {user.email}
                      </p>
                    </div>

                    {/* Navigation Options */}
                    <div className="space-y-0.5">
                      <Link
                        href={getDashboardRoute(user.role)}
                        onClick={() => setDropdownOpen(false)}
                        className="flex items-center gap-2 px-3 py-2 text-xs rounded-[10px] hover:bg-[var(--card-solid-subtle)] transition-colors font-medium text-[var(--ink-primary)]"
                      >
                        <LayoutDashboard className="h-3.5 w-3.5 text-[var(--ink-secondary)]" />
                        <span>My Dashboard</span>
                      </Link>
                    </div>

                    {/* Push Notification Status & Control */}
                    {isSupported && (
                      <div className="mt-1 px-3 py-2 border-t border-[var(--card-line)] flex items-center justify-between text-xs">
                        <span className="flex items-center gap-1.5 text-[var(--ink-secondary)]">
                          <Bell className="h-3.5 w-3.5" />
                          <span>Push Alerts</span>
                        </span>
                        <button
                          type="button"
                          onClick={() =>
                            isSubscribed ? disablePush() : enablePush()
                          }
                          className="text-[10px] uppercase font-bold rounded-full px-2.5 py-0.5 bg-[var(--card-solid-subtle)] border border-[var(--card-line)] text-[var(--ink-primary)] hover:bg-[var(--color-dark)] hover:text-[var(--ink-inverse)] transition-colors cursor-pointer"
                        >
                          {isSubscribed ? "Active" : "Enable"}
                        </button>
                      </div>
                    )}

                    {/* Logout Action */}
                    <div className="pt-1 border-t border-[var(--card-line)] mt-1">
                      <button
                        type="button"
                        onClick={handleLogout}
                        disabled={isLoggingOut}
                        className="w-full flex items-center justify-between px-3 py-2 text-xs rounded-[10px] text-[var(--color-coral)] hover:bg-[var(--color-coral-dim)] transition-colors cursor-pointer font-bold disabled:opacity-50"
                      >
                        <span className="flex items-center gap-2">
                          <LogOut className="h-3.5 w-3.5" />
                          <span>{isLoggingOut ? "Signing out..." : "Log Out"}</span>
                        </span>
                        <span className="text-[10px] uppercase opacity-60">ESC</span>
                      </button>
                    </div>
                  </div>
                )}
              </div>
            </>
          ) : (
            <Link href="/login">
              <Button variant="primary" size="sm" className="text-xs h-8">
                Sign In
              </Button>
            </Link>
          )}
        </div>
      </div>
    </header>
  );
}
