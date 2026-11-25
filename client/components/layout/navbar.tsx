"use client";

import React, { useState, useRef, useEffect } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  LogOut,
  LayoutDashboard,
  Bell,
  Sun,
  Moon,
  Menu,
  Calendar,
  Settings,
  Crown,
} from "lucide-react";
import { NotificationBell } from "@/components/notifications/notification-bell";
import { useAuth } from "@/providers/auth-provider";
import { useTenant } from "@/providers/tenant-provider";
import { useToast } from "@/providers/toast-provider";
import { useNotificationsContext } from "@/providers/notification-provider";
import {
  useEventDetail,
  useTransferEventOwnership,
  useUpdateEvent,
} from "@/hooks/use-events";
import { getAuthenticatedHomeRoute } from "@/lib/auth-routes";
import { apiErrorMessage } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Alert } from "@/components/ui/alert";
import { LogoUploadField } from "@/components/events/logo-upload-field";

export function Navbar() {
  const pathname = usePathname();
  const {
    user,
    isAuthenticated,
    isDev,
    isOwner,
    isAdmin,
    logout,
  } = useAuth();
  const {
    eventSlug,
    activeEvent,
    competitions,
    competitionSlug,
    selectCompetition,
    clearEvent,
  } = useTenant();
  const { isSupported, isSubscribed, enablePush, disablePush } =
    useNotificationsContext();
  const toast = useToast();

  const { data: eventDetail } = useEventDetail(eventSlug, {
    enabled: isAuthenticated && Boolean(eventSlug),
  });

  const updateEvent = useUpdateEvent(eventSlug || "");
  const transferOwnership = useTransferEventOwnership(eventSlug || "");

  const [isLoggingOut, setIsLoggingOut] = useState(false);
  const [dropdownOpen, setDropdownOpen] = useState(false);
  const [isDark, setIsDark] = useState(false);

  // Event Settings / Ownership Transfer Dialog state
  const [eventSettingsOpen, setEventSettingsOpen] = useState(false);
  const [eventName, setEventName] = useState("");
  const [eventDesc, setEventDesc] = useState("");
  const [eventLogoUrl, setEventLogoUrl] = useState("");
  const [newOwnerEmail, setNewOwnerEmail] = useState("");
  const [newOwnerName, setNewOwnerName] = useState("");
  const [eventError, setEventError] = useState<string | null>(null);

  const dropdownRef = useRef<HTMLDivElement>(null);

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
      } catch { }
    } else {
      document.documentElement.classList.remove("dark");
      document.documentElement.classList.add("light");
      try {
        localStorage.setItem("theme", "light");
      } catch { }
    }
  };

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

  const openEventSettings = () => {
    setEventName(eventDetail?.name || activeEvent?.event_name || "");
    setEventDesc(eventDetail?.description || "");
    setEventLogoUrl(eventDetail?.logo_url || activeEvent?.logo_url || "");
    setNewOwnerEmail("");
    setNewOwnerName("");
    setEventError(null);
    setEventSettingsOpen(true);
  };

  const openEventSettingsRef = useRef(openEventSettings);

  useEffect(() => {
    openEventSettingsRef.current = openEventSettings;
  });

  useEffect(() => {
    const handler = () => openEventSettingsRef.current();
    window.addEventListener("cadence:open-event-settings", handler);
    return () =>
      window.removeEventListener("cadence:open-event-settings", handler);
  }, []);

  const handleSaveEventSettings = async (e: React.FormEvent) => {
    e.preventDefault();
    setEventError(null);
    if (!eventName.trim() || eventName.trim().length < 2) {
      setEventError("Event name must be at least 2 characters.");
      return;
    }
    try {
      await updateEvent.mutateAsync({
        name: eventName.trim(),
        description: eventDesc.trim(),
        logo_url: eventLogoUrl.trim(),
      });
      toast.success("Event details updated.", "Event updated");
      setEventSettingsOpen(false);
    } catch (err: unknown) {
      setEventError(apiErrorMessage(err, "Failed to update event"));
    }
  };

  const handleTransferOwnership = async () => {
    setEventError(null);
    if (!newOwnerEmail.trim()) {
      setEventError("Enter the new owner's email address.");
      return;
    }
    try {
      await transferOwnership.mutateAsync({
        new_owner_email: newOwnerEmail.trim(),
        new_owner_name: newOwnerName.trim() || undefined,
      });
      toast.success(
        `Event ownership transferred to ${newOwnerEmail.trim()}.`,
        "Ownership transferred"
      );
      setEventSettingsOpen(false);
    } catch (err: unknown) {
      setEventError(apiErrorMessage(err, "Failed to transfer ownership"));
    }
  };

  const homeHref =
    isAuthenticated && user
      ? getAuthenticatedHomeRoute(user, eventSlug)
      : "/login";

  const [failedAvatarUrl, setFailedAvatarUrl] = useState<string | null>(null);

  const initials = user?.name
    ? user.name
      .split(" ")
      .map((n) => n[0])
      .join("")
      .slice(0, 2)
      .toUpperCase()
    : "CD";

  const showAvatarImage = Boolean(
    user?.avatar_url && failedAvatarUrl !== user.avatar_url
  );

  const isDashboard = pathname.startsWith("/dashboard");
  const displayEventName =
    eventDetail?.name || activeEvent?.event_name || eventSlug;

  return (
    <>
      <header className="app-header font-sans transition-colors duration-200">
        <div className="mx-auto flex h-14 max-w-7xl items-center justify-between gap-2 px-4 sm:px-6">
          {/* Left: Brand, Mobile Sidebar Trigger, and Event/Competition Switcher */}
          <div className="flex items-center gap-2 sm:gap-3 min-w-0">
            {isAuthenticated && user && isDashboard && (
              <button
                type="button"
                onClick={() =>
                  window.dispatchEvent(new CustomEvent("cadence:toggle-sidebar"))
                }
                aria-label="Toggle navigation menu"
                className="md:hidden flex items-center justify-center h-8 w-8 shrink-0 rounded-[6px] bg-[var(--card-solid-subtle)] border border-[var(--card-line)] text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] active:scale-95 transition-transform cursor-pointer"
              >
                <Menu className="h-4 w-4" />
              </button>
            )}

            <Link
              href={homeHref}
              className="flex items-center gap-2 text-decoration-none group select-none shrink-0"
            >
              <span className="text-base font-bold tracking-tight text-[var(--ink-primary)]">
                Cadence
                <span className="hidden lg:inline ml-1.5 text-xs font-medium tracking-normal text-[var(--ink-tertiary)]">
                  by DevClub
                </span>
              </span>
            </Link>

            {isAuthenticated && isDashboard && eventSlug && (
              <div className="hidden sm:flex items-center gap-1.5 min-w-0 pl-2 border-l border-[var(--card-line)]">
                <Link
                  href="/events"
                  title="Switch event"
                  className="text-xs font-medium text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] truncate max-w-[160px] transition-colors"
                >
                  {displayEventName}
                </Link>
                {competitions.length > 1 && (
                  <>
                    <span className="text-xs text-[var(--ink-tertiary)]">/</span>
                    <select
                      aria-label="Active competition"
                      value={competitionSlug ?? ""}
                      onChange={(e) => selectCompetition(e.target.value || null)}
                      className="h-7 rounded-[6px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] px-2 text-xs font-semibold text-[var(--ink-primary)] focus:outline-none focus:border-[var(--color-blue)] cursor-pointer max-w-[180px] truncate"
                    >
                      {competitions.map((comp) => (
                        <option key={comp.id} value={comp.slug}>
                          {comp.name}
                        </option>
                      ))}
                    </select>
                  </>
                )}
              </div>
            )}
          </div>

          {/* Right Section: Theme Switcher, Notification Bell & User Avatar */}
          <div className="flex items-center gap-2 shrink-0">
            <button
              type="button"
              onClick={toggleTheme}
              aria-label="Toggle light/dark theme"
              className="circle-icon-btn !w-8 !h-8 text-[var(--ink-secondary)] hover:text-[var(--ink-primary)]"
            >
              {isDark ? (
                <Sun className="h-3.5 w-3.5" />
              ) : (
                <Moon className="h-3.5 w-3.5" />
              )}
            </button>

            {isAuthenticated && user ? (
              <>
                {eventSlug && <NotificationBell />}

                {!pathname.startsWith("/dashboard") && (
                <div className="relative" ref={dropdownRef}>
                  <button
                    type="button"
                    onClick={() => setDropdownOpen((prev) => !prev)}
                    aria-label="User menu"
                    aria-expanded={dropdownOpen}
                    className="flex items-center justify-center h-8 w-8 rounded-[8px] overflow-hidden bg-[var(--card-solid-subtle)] border border-[var(--card-line)] text-[var(--ink-primary)] font-semibold text-xs shadow-xs hover:border-[var(--color-blue)] transition-[border-color,transform] active:scale-95 cursor-pointer select-none"
                  >
                    {showAvatarImage ? (
                      // eslint-disable-next-line @next/next/no-img-element
                      <img
                        src={user.avatar_url!}
                        alt={user.name || "User avatar"}
                        referrerPolicy="no-referrer"
                        onError={() => setFailedAvatarUrl(user.avatar_url ?? null)}
                        className="h-full w-full object-cover"
                      />
                    ) : (
                      initials
                    )}
                  </button>

                  {dropdownOpen && (() => {
                    const showDashboard = Boolean(eventSlug || isDev);
                    const showSwitchEvent = pathname !== "/events";
                    const showEventSettings = Boolean(eventSlug && isAdmin);
                    const hasMenuActions = showDashboard || showSwitchEvent || showEventSettings;
                    const showPushAlerts = Boolean(isSupported && eventSlug);
                    return (
                    <div className="absolute right-0 mt-2 w-64 origin-top-right rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid)] shadow-[var(--shadow-container)] z-50 animate-in fade-in-0 zoom-in-95 duration-150 p-2 text-[var(--ink-primary)] font-sans">
                      {hasMenuActions && (
                      <div className="space-y-0.5">
                        {showDashboard && (
                          <Link
                            href={
                              pathname === "/events" && isDev
                                ? "/dashboard/dev"
                                : getAuthenticatedHomeRoute(user, eventSlug)
                            }
                            onClick={() => {
                              if (pathname === "/events" && isDev) {
                                clearEvent();
                              }
                              setDropdownOpen(false);
                            }}
                            className="flex items-center gap-2 px-3 py-2 text-xs rounded-[6px] hover:bg-[var(--card-solid-subtle)] transition-colors font-medium text-[var(--ink-primary)]"
                          >
                            <LayoutDashboard className="h-3.5 w-3.5 text-[var(--ink-secondary)]" />
                            <span>Dashboard</span>
                          </Link>
                        )}
                        {showSwitchEvent && (
                          <Link
                            href="/events"
                            onClick={() => setDropdownOpen(false)}
                            className="flex items-center gap-2 px-3 py-2 text-xs rounded-[6px] hover:bg-[var(--card-solid-subtle)] transition-colors font-medium text-[var(--ink-primary)]"
                          >
                            <Calendar className="h-3.5 w-3.5 text-[var(--ink-secondary)]" />
                            <span>Switch event</span>
                          </Link>
                        )}
                        {showEventSettings && (
                          <button
                            type="button"
                            onClick={() => {
                              setDropdownOpen(false);
                              openEventSettings();
                            }}
                            className="w-full flex items-center gap-2 px-3 py-2 text-xs rounded-[6px] hover:bg-[var(--card-solid-subtle)] transition-colors font-medium text-[var(--ink-primary)] cursor-pointer"
                          >
                            <Settings className="h-3.5 w-3.5 text-[var(--ink-secondary)]" />
                            <span>Event settings</span>
                          </button>
                        )}
                      </div>
                      )}

                      {showPushAlerts && (
                        <div className={`${hasMenuActions ? "mt-1 border-t border-[var(--card-line)] " : ""}px-3 py-2 flex items-center justify-between text-xs`}>
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

                      <div className={hasMenuActions || showPushAlerts ? "pt-1 border-t border-[var(--card-line)] mt-1" : ""}>
                        <button
                          type="button"
                          onClick={handleLogout}
                          disabled={isLoggingOut}
                          className="w-full flex items-center justify-between px-3 py-2 text-xs rounded-[6px] text-[var(--color-coral)] hover:bg-[var(--color-coral-dim)] transition-colors cursor-pointer font-medium disabled:opacity-50"
                        >
                          <span className="flex items-center gap-2">
                            <LogOut className="h-3.5 w-3.5" />
                            <span>
                              {isLoggingOut ? "Signing out..." : "Log out"}
                            </span>
                          </span>
                          <span className="text-[10px] uppercase opacity-60">
                            ESC
                          </span>
                        </button>
                      </div>
                    </div>
                    );
                  })()}
                </div>
                )}
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

      {/* Event Settings & Ownership Transfer Dialog */}
      <Dialog
        open={eventSettingsOpen}
        onClose={() => setEventSettingsOpen(false)}
        title="Event Settings"
        description={`Manage metadata${isOwner ? " and ownership" : ""} for ${displayEventName}.`}
        footer={
          <div className="flex justify-end gap-2">
            <Button
              variant="outline"
              size="sm"
              type="button"
              onClick={() => setEventSettingsOpen(false)}
              disabled={updateEvent.isPending || transferOwnership.isPending}
            >
              Close
            </Button>
            <Button
              variant="primary"
              size="sm"
              type="button"
              onClick={handleSaveEventSettings}
              isLoading={updateEvent.isPending}
            >
              Save Event Details
            </Button>
          </div>
        }
      >
        <div className="space-y-4 font-sans text-xs">
          {eventError && (
            <Alert variant="error" title="Update failed">
              {eventError}
            </Alert>
          )}

          <form onSubmit={handleSaveEventSettings} className="space-y-3">
            <Input
              label="Event Name *"
              value={eventName}
              onChange={(e) => setEventName(e.target.value)}
              required
            />
            <Textarea
              label="Description"
              rows={2}
              value={eventDesc}
              onChange={(e) => setEventDesc(e.target.value)}
            />
            <LogoUploadField
              value={eventLogoUrl}
              onChange={setEventLogoUrl}
              disabled={updateEvent.isPending}
            />
          </form>

          {isOwner && (
            <div className="pt-3 border-t border-[var(--card-line)] space-y-2.5">
              <div className="flex items-center gap-1.5 font-semibold text-[var(--ink-primary)]">
                <Crown className="h-3.5 w-3.5 text-[var(--color-amber)]" />
                <span>Transfer Event Ownership</span>
              </div>
              <p className="text-[11px] text-[var(--ink-secondary)]">
                Transferring ownership assigns the target user as the sole owner of this event and converts your role to admin.
              </p>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                <Input
                  label="New Owner Email *"
                  type="email"
                  placeholder="owner@event.com"
                  value={newOwnerEmail}
                  onChange={(e) => setNewOwnerEmail(e.target.value)}
                />
                <Input
                  label="New Owner Name (optional)"
                  placeholder="Name"
                  value={newOwnerName}
                  onChange={(e) => setNewOwnerName(e.target.value)}
                />
              </div>
              <div className="flex justify-end">
                <Button
                  variant="outline"
                  size="sm"
                  type="button"
                  onClick={handleTransferOwnership}
                  isLoading={transferOwnership.isPending}
                  className="text-[var(--color-coral)]"
                >
                  Transfer Ownership
                </Button>
              </div>
            </div>
          )}
        </div>
      </Dialog>
    </>
  );
}
