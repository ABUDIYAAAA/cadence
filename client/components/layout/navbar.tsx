"use client";

import React, { useState, useRef, useEffect } from "react";
import Link from "next/link";
import { User, LogOut, LayoutDashboard, Bell } from "lucide-react";
import { NotificationBell } from "@/components/notifications/notification-bell";
import { useAuth } from "@/providers/auth-provider";
import { useToast } from "@/providers/toast-provider";
import { useNotificationsContext } from "@/providers/notification-provider";
import { getDashboardRoute } from "@/lib/auth-routes";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

export function Navbar() {
  const { user, isAuthenticated, logout } = useAuth();
  const { isSupported, isSubscribed, enablePush, disablePush } =
    useNotificationsContext();
  const toast = useToast();
  const [isLoggingOut, setIsLoggingOut] = useState(false);
  const [dropdownOpen, setDropdownOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

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

  return (
    <header className="border-b border-black bg-white font-mono sticky top-0 z-40">
      <div className="mx-auto flex h-14 max-w-7xl items-center justify-between px-4 sm:px-6">
        {/* Left Section: Pure Bold Black Text Logo */}
        <div className="flex items-center gap-6">
          <Link
            href={homeHref}
            className="text-lg font-black tracking-widest text-black uppercase select-none hover:opacity-80 transition-opacity"
          >
            CADENCE
          </Link>

          {/* Navigation Links */}
          {isAuthenticated && user && (
            <nav className="hidden md:flex items-center gap-4 text-xs uppercase tracking-wider">
              <Link
                href={getDashboardRoute(user.role)}
                className="text-black font-semibold transition-colors hover:underline"
              >
                Dashboard
              </Link>
            </nav>
          )}
        </div>

        {/* Right Section: Notification Bell + User Icon Dropdown */}
        <div className="flex items-center gap-2.5">
          {isAuthenticated && user ? (
            <>
              {/* Notification Bell with Unread Dot & Feed Dropdown */}
              <NotificationBell />

              <div className="relative" ref={dropdownRef}>
              <button
                type="button"
                onClick={() => setDropdownOpen((prev) => !prev)}
                aria-label="User menu"
                aria-expanded={dropdownOpen}
                className="flex items-center justify-center h-9 w-9 border border-black bg-white hover:bg-neutral-100 active:bg-neutral-200 transition-colors cursor-pointer select-none"
              >
                <User className="h-4 w-4 text-black" />
              </button>

              {/* Dropdown Menu (anchored right) */}
              {dropdownOpen && (
                <div className="absolute right-0 mt-2 w-64 border-2 border-black bg-white shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] z-50 animate-in fade-in-0 zoom-in-95 text-black">
                  {/* User Profile Header */}
                  <div className="p-3 bg-neutral-50 border-b border-neutral-200">
                    <div className="flex items-center justify-between mb-1">
                      <span className="text-[10px] uppercase font-bold text-neutral-500 tracking-wider">
                        Active Account
                      </span>
                      <Badge variant="solid" size="sm">
                        {user.role}
                      </Badge>
                    </div>
                    <p className="text-xs font-bold text-black truncate">
                      {user.name}
                    </p>
                    <p className="text-[11px] text-neutral-600 truncate font-mono">
                      {user.email}
                    </p>
                  </div>

                  {/* Navigation Options */}
                  <div className="p-1">
                    <Link
                      href={getDashboardRoute(user.role)}
                      onClick={() => setDropdownOpen(false)}
                      className="flex items-center gap-2 px-3 py-2 text-xs hover:bg-neutral-100 transition-colors font-medium text-black"
                    >
                      <LayoutDashboard className="h-3.5 w-3.5 text-neutral-600" />
                      <span>My Dashboard</span>
                    </Link>
                  </div>

                  {/* Push Notification Status & Control */}
                  {isSupported && (
                    <div className="px-3 py-2 border-t border-neutral-200 flex items-center justify-between text-xs bg-neutral-50">
                      <span className="flex items-center gap-1.5 text-neutral-700">
                        <Bell className="h-3.5 w-3.5" />
                        <span>Push Alerts</span>
                      </span>
                      <button
                        type="button"
                        onClick={() =>
                          isSubscribed ? disablePush() : enablePush()
                        }
                        className="text-[10px] uppercase font-bold border border-black px-2 py-0.5 hover:bg-black hover:text-white transition-colors cursor-pointer"
                      >
                        {isSubscribed ? "Active" : "Enable"}
                      </button>
                    </div>
                  )}

                  {/* Logout Action */}
                  <div className="p-1 border-t border-neutral-200">
                    <button
                      type="button"
                      onClick={handleLogout}
                      disabled={isLoggingOut}
                      className="w-full flex items-center justify-between px-3 py-2 text-xs hover:bg-black hover:text-white transition-colors cursor-pointer font-bold disabled:opacity-50"
                    >
                      <span className="flex items-center gap-2">
                        <LogOut className="h-3.5 w-3.5" />
                        <span>{isLoggingOut ? "Signing out..." : "Log Out"}</span>
                      </span>
                      <span className="text-[10px] uppercase">ESC</span>
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
