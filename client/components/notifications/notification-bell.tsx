"use client";

import React, { useState, useRef, useEffect } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Bell, Check, ExternalLink, CheckCheck } from "lucide-react";
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
    <div className="relative font-mono" ref={dropdownRef}>
      {/* Bell Trigger Button with Unread Dot */}
      <button
        type="button"
        onClick={() => setIsOpen((prev) => !prev)}
        aria-label="View notifications"
        aria-expanded={isOpen}
        className="relative flex items-center justify-center h-9 w-9 border border-black bg-white hover:bg-neutral-100 active:bg-neutral-200 transition-colors cursor-pointer select-none"
      >
        <Bell className="h-4 w-4 text-black" />

        {/* Unread indicator dot */}
        {unreadCount > 0 && (
          <span className="absolute top-1.5 right-1.5 flex h-2.5 w-2.5">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-black opacity-75" />
            <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-black border border-white" />
          </span>
        )}
      </button>

      {/* Notification Dropdown Menu */}
      {isOpen && (
        <div className="absolute right-0 mt-2 w-80 sm:w-96 border-2 border-black bg-white shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] z-50 animate-in fade-in-0 zoom-in-95 text-black">
          {/* Header */}
          <div className="p-3 bg-neutral-50 border-b border-black flex items-center justify-between">
            <div className="flex items-center gap-2">
              <span className="text-xs font-bold uppercase tracking-wider text-black">
                Notifications
              </span>
              {unreadCount > 0 ? (
                <Badge variant="solid" size="sm">
                  {unreadCount} NEW
                </Badge>
              ) : (
                <Badge variant="subtle" size="sm">
                  0 NEW
                </Badge>
              )}
            </div>

            {unreadCount > 0 && (
              <Button
                variant="ghost"
                size="sm"
                onClick={handleMarkAllRead}
                isLoading={markAllAsReadMutation.isPending}
                className="h-6 px-2 text-[10px] gap-1 hover:bg-neutral-200 text-black uppercase font-bold"
              >
                <CheckCheck className="h-3 w-3" />
                <span>Mark All Read</span>
              </Button>
            )}
          </div>

          {/* List of in-app notifications */}
          <div className="max-h-80 overflow-y-auto divide-y divide-neutral-200">
            {isLoading ? (
              <div className="p-6 text-center text-xs text-neutral-500">
                Checking notification feed...
              </div>
            ) : notifications.length === 0 ? (
              <div className="p-8 text-center space-y-1">
                <span className="text-lg text-neutral-400 block">[—]</span>
                <p className="text-xs font-bold uppercase text-neutral-700">
                  Inbox is Clear
                </p>
                <p className="text-[11px] text-neutral-500">
                  No notifications or announcements received yet.
                </p>
              </div>
            ) : (
              notifications.map((item) => (
                <div
                  key={item.id}
                  onClick={() => handleNotificationClick(item)}
                  className={`p-3 text-left transition-colors cursor-pointer flex items-start gap-2.5 ${
                    !item.is_read
                      ? "bg-neutral-50 hover:bg-neutral-100 font-medium"
                      : "bg-white hover:bg-neutral-50 opacity-80"
                  }`}
                >
                  {/* Unread indicator bullet */}
                  <div className="pt-1 shrink-0">
                    {!item.is_read ? (
                      <span className="block h-2 w-2 rounded-full bg-black" />
                    ) : (
                      <span className="block h-2 w-2 rounded-full border border-neutral-300" />
                    )}
                  </div>

                  <div className="flex-1 min-w-0 space-y-1">
                    <div className="flex items-center justify-between gap-2">
                      <h5 className="text-xs font-bold text-black truncate">
                        {item.title}
                      </h5>
                      <span className="text-[10px] text-neutral-500 shrink-0 whitespace-nowrap">
                        {formatRelativeTime(item.created_at)}
                      </span>
                    </div>

                    <p className="text-[11px] text-neutral-700 leading-snug line-clamp-2">
                      {item.body}
                    </p>

                    {Boolean(
                      item.action_url &&
                        item.action_url.trim() &&
                        item.action_url !== "/"
                    ) && (
                      <div className="pt-1 flex items-center gap-1 text-[10px] font-bold text-black underline">
                        <span>Open link</span>
                        <ExternalLink className="h-2.5 w-2.5" />
                      </div>
                    )}
                  </div>
                </div>
              ))
            )}
          </div>

          {/* Dropdown Footer */}
          <div className="p-2 border-t border-neutral-200 bg-neutral-50 text-center">
            <span className="text-[10px] text-neutral-500 uppercase tracking-wider">
              Press ESC or click outside to dismiss
            </span>
          </div>
        </div>
      )}
    </div>
  );
}
