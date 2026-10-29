"use client";

import React, { useState, useRef, useEffect } from "react";
import { useRouter } from "next/navigation";
import { Bell, ExternalLink, CheckCheck } from "lucide-react";
import {
  useNotificationInbox,
  useMarkNotificationAsRead,
  useMarkAllNotificationsAsRead,
} from "@/hooks/use-notifications";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { NotificationItem } from "@/types/notifications";

function formatRelativeTime(dateString: string): string {
  try {
    const now = Date.now();
    const date = new Date(dateString).getTime();
    const diffSeconds = Math.floor((now - date) / 1000);

    if (diffSeconds < 60) return "Just now";
    const diffMinutes = Math.floor(diffSeconds / 60);
    if (diffMinutes < 60) return `${diffMinutes}m ago`;
    const diffHours = Math.floor(diffMinutes / 60);
    if (diffHours < 24) return `${diffHours}h ago`;
    const diffDays = Math.floor(diffHours / 24);
    return `${diffDays}d ago`;
  } catch {
    return "";
  }
}

export function NotificationBell() {
  const router = useRouter();
  const [isOpen, setIsOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  const { data: inboxData, isLoading } = useNotificationInbox(1, 20);
  const markAsReadMutation = useMarkNotificationAsRead();
  const markAllAsReadMutation = useMarkAllNotificationsAsRead();

  const notifications = inboxData?.notifications || [];
  const unreadCount = inboxData?.unread_count || 0;

  // Handle click outside & escape key
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(event.target as Node)
      ) {
        setIsOpen(false);
      }
    };

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setIsOpen(false);
      }
    };

    if (isOpen) {
      document.addEventListener("mousedown", handleClickOutside);
      document.addEventListener("keydown", handleKeyDown);
    }

    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [isOpen]);

  const handleNotificationClick = (item: NotificationItem) => {
    if (!item.is_read) {
      markAsReadMutation.mutate(item.id);
    }
    if (item.action_url && item.action_url.trim() && item.action_url !== "/") {
      setIsOpen(false);
      router.push(item.action_url);
    }
  };

  const handleMarkAllRead = async () => {
    try {
      await markAllAsReadMutation.mutateAsync();
    } catch (err) {
      console.warn("Failed to mark all notifications as read:", err);
    }
  };

  return (
    <div className="relative font-sans" ref={dropdownRef}>
      {/* Bell Trigger Button with Unread Dot */}
      <button
        type="button"
        onClick={() => setIsOpen((prev) => !prev)}
        aria-label="View notifications"
        aria-expanded={isOpen}
        className="circle-icon-btn !w-8 !h-8 relative text-[var(--ink-secondary)] hover:text-[var(--ink-primary)]"
      >
        <Bell className="h-4 w-4" />

        {/* Unread indicator dot */}
        {unreadCount > 0 && (
          <span className="absolute top-1 right-1 flex h-2 w-2">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-[var(--color-coral)] opacity-75" />
            <span className="relative inline-flex rounded-full h-2 w-2 bg-[var(--color-coral)] ring-2 ring-[var(--card-solid)]" />
          </span>
        )}
      </button>

      {/* Notification Dropdown Menu */}
      {isOpen && (
        <div className="absolute right-0 mt-2 w-80 sm:w-96 rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid)] shadow-[var(--shadow-container)] z-50 animate-in fade-in-0 zoom-in-95 duration-150 p-2 text-[var(--ink-primary)]">
          {/* Header */}
          <div className="p-2.5 bg-[var(--card-solid-subtle)] rounded-[8px] border border-[var(--card-line)] flex items-center justify-between mb-1.5">
            <div className="flex items-center gap-2">
              <span className="text-xs font-semibold text-[var(--ink-primary)]">
                Notifications
              </span>
              {unreadCount > 0 ? (
                <Badge variant="blue" size="sm">
                  {unreadCount} new
                </Badge>
              ) : (
                <Badge variant="subtle" size="sm">
                  0 new
                </Badge>
              )}
            </div>

            {unreadCount > 0 && (
              <Button
                variant="ghost"
                size="sm"
                onClick={handleMarkAllRead}
                isLoading={markAllAsReadMutation.isPending}
                className="h-6 px-2 text-[11px] gap-1 rounded-[6px] text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] font-medium"
              >
                <CheckCheck className="h-3 w-3" />
                <span>Mark all read</span>
              </Button>
            )}
          </div>

          {/* List of in-app notifications */}
          <div className="max-h-80 overflow-y-auto space-y-1">
            {isLoading ? (
              <div className="p-6 text-center text-xs text-[var(--ink-secondary)]">
                Loading notifications...
              </div>
            ) : notifications.length === 0 ? (
              <div className="p-8 text-center space-y-1">
                <p className="text-xs font-semibold text-[var(--ink-secondary)]">
                  No notifications yet
                </p>
                <p className="text-[11px] text-[var(--ink-tertiary)]">
                  Announcements and alerts will appear here.
                </p>
              </div>
            ) : (
              notifications.map((item) => (
                <div
                  key={item.id}
                  onClick={() => handleNotificationClick(item)}
                  className={`p-2.5 rounded-[8px] text-left transition-colors cursor-pointer flex items-start gap-2.5 ${
                    !item.is_read
                      ? "bg-[var(--color-blue-dim)] hover:bg-[var(--color-blue-dim)]/80"
                      : "hover:bg-[var(--card-solid-subtle)] opacity-80"
                  }`}
                >
                  {/* Unread indicator bullet */}
                  <div className="pt-1.5 shrink-0">
                    {!item.is_read ? (
                      <span className="block h-1.5 w-1.5 rounded-full bg-[var(--color-blue)]" />
                    ) : (
                      <span className="block h-1.5 w-1.5 rounded-full bg-[var(--card-line)]" />
                    )}
                  </div>

                  <div className="flex-1 min-w-0 space-y-0.5">
                    <div className="flex items-center justify-between gap-2">
                      <h5 className="text-xs font-semibold text-[var(--ink-primary)] truncate">
                        {item.title}
                      </h5>
                      <span className="text-[10px] text-[var(--ink-tertiary)] shrink-0 whitespace-nowrap">
                        {formatRelativeTime(item.created_at)}
                      </span>
                    </div>

                    <p className="text-[11px] text-[var(--ink-secondary)] leading-snug line-clamp-2">
                      {item.body}
                    </p>

                    {Boolean(
                      item.action_url &&
                        item.action_url.trim() &&
                        item.action_url !== "/"
                    ) && (
                      <div className="pt-1 flex items-center gap-1 text-[11px] font-medium text-[var(--color-blue)] hover:underline">
                        <span>Open link</span>
                        <ExternalLink className="h-2.5 w-2.5" />
                      </div>
                    )}
                  </div>
                </div>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  );
}
