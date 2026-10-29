"use client";

import React, { useState, useRef, useEffect } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { LogOut, LayoutDashboard, Bell, Sun, Moon, Menu } from "lucide-react";
import { NotificationBell } from "@/components/notifications/notification-bell";
import { useAuth } from "@/providers/auth-provider";
import { useToast } from "@/providers/toast-provider";
import { useNotificationsContext } from "@/providers/notification-provider";
import { getDashboardRoute } from "@/lib/auth-routes";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

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

  // Sync dark mode state with document class asynchronously
  useEffect(() => {
    const isDarkMode = document.documentElement.classList.contains("dark");
    const frame = requestAnimationFrame(() => {
      setIsDark(isDarkMode);
    });
    return () => cancelAnimationFrame(frame);
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
      toast.info("You have been signed out.", "Session Ended");
    } finally {
      setIsLoggingOut(false);
    }
  };

  const homeHref = isAuthenticated && user ? getDashboardRoute(user.role) : "/login";

  // Compute initials
  const initials = user?.name
    ? user.name
        .split(" ")
        .map((n) => n[0])
        .join("")
        .slice(0, 2)
        .toUpperCase()
    : "CD";

  const isDashboard = pathname.startsWith("/dashboard");

  return (
    <header className="app-header font-sans transition-colors duration-200">
      <div className="mx-auto flex h-14 max-w-7xl items-center justify-between px-4 sm:px-6">
        {/* Brand & Mobile Sidebar Trigger */}
        <div className="flex items-center gap-2 sm:gap-3">
          {isAuthenticated && user && isDashboard && (
            <button
              type="button"
              onClick={() =>
                window.dispatchEvent(new CustomEvent("cadence:toggle-sidebar"))
              }
              aria-label="Toggle navigation menu"
              className="md:hidden flex items-center justify-center h-8 w-8 rounded-[6px] bg-[var(--card-solid-subtle)] border border-[var(--card-line)] text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] active:scale-95 transition-transform cursor-pointer"
            >
              <Menu className="h-4 w-4" />
            </button>
          )}

          <Link
            href={homeHref}
            className="flex items-center gap-2.5 text-decoration-none group select-none"
          >
            <span className="w-6 h-6 rounded-[6px] bg-[var(--color-dark)] text-[var(--ink-inverse)] font-bold text-xs flex items-center justify-center shadow-xs">
              C
            </span>
            <span className="text-base font-bold tracking-tight text-[var(--ink-primary)]">
              Cadence
            </span>
          </Link>
        </div>

        {/* Right Section: Theme Switcher, Notification Bell & User Avatar */}
        <div className="flex items-center gap-2">
          {/* Theme Switcher Button */}
          <button
            type="button"
            onClick={toggleTheme}
            aria-label="Toggle light/dark theme"
            className="circle-icon-btn !w-8 !h-8 text-[var(--ink-secondary)] hover:text-[var(--ink-primary)]"
          >
            {isDark ? <Sun className="h-3.5 w-3.5" /> : <Moon className="h-3.5 w-3.5" />}
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
                  className="flex items-center justify-center h-8 w-8 rounded-[8px] bg-[var(--card-solid-subtle)] border border-[var(--card-line)] text-[var(--ink-primary)] font-semibold text-xs shadow-xs hover:border-[var(--color-blue)] transition-[border-color,transform] active:scale-95 cursor-pointer select-none"
                >
                  {initials}
                </button>

                {/* Dropdown Menu */}
                {dropdownOpen && (
                  <div className="absolute right-0 mt-2 w-64 origin-top-right rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid)] shadow-[var(--shadow-container)] z-50 animate-in fade-in-0 zoom-in-95 duration-150 p-2 text-[var(--ink-primary)] font-sans">
                    {/* User Profile Header */}
                    <div className="p-3 bg-[var(--card-solid-subtle)] rounded-[8px] border border-[var(--card-line)] mb-1.5">
                      <div className="flex items-center justify-between mb-1">
                        <span className="text-[11px] font-medium text-[var(--ink-secondary)]">
                          Account
                        </span>
                        <Badge variant="subtle" size="sm">
                          {user.role}
                        </Badge>
                      </div>
                      <p className="text-xs font-semibold text-[var(--ink-primary)] truncate">
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
                        className="flex items-center gap-2 px-3 py-2 text-xs rounded-[6px] hover:bg-[var(--card-solid-subtle)] transition-colors font-medium text-[var(--ink-primary)]"
                      >
                        <LayoutDashboard className="h-3.5 w-3.5 text-[var(--ink-secondary)]" />
                        <span>Dashboard</span>
                      </Link>
                    </div>

                    {/* Push Notification Status & Control */}
                    {isSupported && (
                      <div className="mt-1 px-3 py-2 border-t border-[var(--card-line)] flex items-center justify-between text-xs">
                        <span className="flex items-center gap-1.5 text-[var(--ink-secondary)]">
                          <Bell className="h-3.5 w-3.5" />
                          <span>Push alerts</span>
                        </span>
                        <button
                          type="button"
                          onClick={() =>
                            isSubscribed ? disablePush() : enablePush()
                          }
                          className="text-[11px] font-medium rounded-[6px] px-2.5 py-0.5 bg-[var(--card-solid-subtle)] border border-[var(--card-line)] text-[var(--ink-primary)] hover:bg-[var(--card-solid)] transition-colors cursor-pointer"
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
                        className="w-full flex items-center justify-between px-3 py-2 text-xs rounded-[6px] text-[var(--color-coral)] hover:bg-[var(--color-coral-dim)] transition-colors cursor-pointer font-medium disabled:opacity-50"
                      >
                        <span className="flex items-center gap-2">
                          <LogOut className="h-3.5 w-3.5" />
                          <span>{isLoggingOut ? "Signing out..." : "Log out"}</span>
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
                Sign in
              </Button>
            </Link>
          )}
        </div>
      </div>
    </header>
  );
}
