"use client";

import React from "react";
import { Search, X, RotateCcw, Filter } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import { AuditLogFilter } from "@/types/audit";

interface AuditFiltersProps {
  filters: AuditLogFilter;
  onChange: (filters: AuditLogFilter) => void;
  onReset: () => void;
}

const ACTION_OPTIONS = [
  { value: "", label: "All Actions" },
  // Auth
  { value: "auth.login_success", label: "[Auth] Login Success" },
  { value: "auth.login_failure", label: "[Auth] Login Failure" },
  { value: "auth.logout", label: "[Auth] Logout" },
  { value: "auth.password_setup", label: "[Auth] Password Setup" },
  { value: "auth.password_forgot", label: "[Auth] Password Reset Requested" },
  { value: "auth.password_reset", label: "[Auth] Password Reset" },
  // User
  { value: "user.create", label: "[User] Account Created" },
  { value: "user.bulk_import", label: "[User] Bulk Imported" },
  { value: "user.update", label: "[User] Record Updated" },
  { value: "user.delete", label: "[User] Account Deleted" },
  { value: "user.send_invite", label: "[User] Invite Dispatched" },
  { value: "user.reset_trigger", label: "[User] Reset Triggered" },
  // Team
  { value: "team.create", label: "[Team] Team Created" },
  { value: "team.update", label: "[Team] Team Updated" },
  { value: "team.delete", label: "[Team] Team Deleted" },
  { value: "team.add_member", label: "[Team] Member Added" },
  { value: "team.remove_member", label: "[Team] Member Removed" },
  // Ticket
  { value: "ticket.issued", label: "[Ticket] Pass Emailed" },
  { value: "ticket.released", label: "[Ticket] Passes Released" },
  { value: "ticket.check_in", label: "[Ticket] Check-In Recorded" },
  { value: "ticket.duplicate_scan", label: "[Ticket] Duplicate Scan" },
  { value: "ticket.scan_rejected", label: "[Ticket] Scan Refused" },
  { value: "ticket.session_create", label: "[Ticket] Scan Session Created" },
  { value: "ticket.session_toggle", label: "[Ticket] Scan Session Opened/Closed" },
  { value: "ticket.session_scan", label: "[Ticket] Attendance/Exit Scan" },
  { value: "ticket.settings_update", label: "[Ticket] Internal Domains Changed" },
  // Judging
  { value: "judging.round_create", label: "[Judging] Round Created" },
  { value: "judging.round_activate", label: "[Judging] Round Activated" },
  { value: "judging.round_end", label: "[Judging] Round Ended" },
  { value: "judging.round_delete", label: "[Judging] Round Deleted" },
  { value: "judging.panel_create", label: "[Judging] Panel Created" },
  { value: "judging.slots_configure", label: "[Judging] Slots Configured" },
  { value: "judging.slot_start", label: "[Judging] Slot Started" },
  { value: "judging.timer_pause", label: "[Judging] Timer Paused" },
  { value: "judging.timer_resume", label: "[Judging] Timer Resumed" },
  { value: "judging.timer_extend", label: "[Judging] Timer Extended" },
  { value: "judging.score_lock", label: "[Judging] Score Locked" },
  { value: "judging.slot_complete", label: "[Judging] Slot Completed" },
  { value: "judging.slot_move", label: "[Judging] Slot Moved" },
  { value: "judging.slot_queue", label: "[Judging] Slot Queued" },
  { value: "judging.slot_unqueue", label: "[Judging] Slot Unqueued" },
  { value: "judging.slot_links_update", label: "[Judging] Submission Links Updated" },
  // Notification
  { value: "notification.broadcast", label: "[Notification] Broadcast Sent" },
  { value: "notification.subscribe", label: "[Notification] Push Subscribed" },
  { value: "notification.unsubscribe", label: "[Notification] Push Unsubscribed" },
  // System
  { value: "system.backup_failed", label: "[System] Backup Failed" },
  { value: "system.backup_recovered", label: "[System] Backup Recovered" },
];

const TARGET_TYPE_OPTIONS = [
  { value: "", label: "All Target Types" },
  { value: "user", label: "User" },
  { value: "team", label: "Team" },
  { value: "ticket", label: "Ticket" },
  { value: "scan_session", label: "Scan Session" },
  { value: "session", label: "Session" },
  { value: "auth", label: "Auth" },
  { value: "judging_round", label: "Judging Round" },
  { value: "judging_slot", label: "Judging Slot" },
  { value: "judging_panel", label: "Judging Panel" },
  { value: "notification", label: "Notification" },
  { value: "system", label: "System" },
];

export function AuditFilters({ filters, onChange, onReset }: AuditFiltersProps) {
  const hasActiveFilters = Boolean(
    filters.search ||
      filters.action ||
      filters.target_type ||
      filters.from_date ||
      filters.to_date
  );

  return (
    <div className="space-y-3 font-sans">
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-3">
        {/* Search */}
        <div className="relative lg:col-span-2">
          <Input
            placeholder="Search description, email, IP, or ID..."
            value={filters.search || ""}
            onChange={(e) =>
              onChange({ ...filters, search: e.target.value, page: 1 })
            }
            className="pl-8 text-xs"
          />
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-[var(--ink-tertiary)] pointer-events-none" />
          {filters.search && (
            <button
              onClick={() => onChange({ ...filters, search: "", page: 1 })}
              className="absolute right-2.5 top-2.5 text-[var(--ink-tertiary)] hover:text-[var(--ink-primary)] cursor-pointer"
              type="button"
            >
              <X className="h-4 w-4" />
            </button>
          )}
        </div>

        {/* Action Dropdown */}
        <div>
          <Select
            value={filters.action || ""}
            onChange={(e) =>
              onChange({
                ...filters,
                action: e.target.value || undefined,
                page: 1,
              })
            }
            className="text-xs"
          >
            {ACTION_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </Select>
        </div>

        {/* Target Type Dropdown */}
        <div>
          <Select
            value={filters.target_type || ""}
            onChange={(e) =>
              onChange({
                ...filters,
                target_type: e.target.value || undefined,
                page: 1,
              })
            }
            className="text-xs"
          >
            {TARGET_TYPE_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </Select>
        </div>

        {/* Reset Filter Button */}
        <div className="flex items-center">
          {hasActiveFilters ? (
            <Button
              variant="outline"
              size="sm"
              onClick={onReset}
              className="h-9 w-full text-xs gap-1.5"
            >
              <RotateCcw className="h-3.5 w-3.5" />
              Reset Filters
            </Button>
          ) : (
            <div className="hidden lg:flex items-center text-[10px] text-[var(--ink-tertiary)] gap-1 pl-1">
              <Filter className="h-3 w-3" />
              <span>FILTER ACTIVE TRAIL</span>
            </div>
          )}
        </div>
      </div>

      {/* Date Range Sub-Bar */}
      <div className="flex flex-wrap items-center gap-3 pt-2 border-t border-[var(--card-line)] text-xs">
        <span className="text-[var(--ink-secondary)] font-semibold text-[11px] uppercase">
          Timestamp Range:
        </span>
        <div className="flex items-center gap-2">
          <span className="text-[var(--ink-tertiary)] text-[10px]">From</span>
          <input
            type="date"
            value={filters.from_date ? filters.from_date.split("T")[0] : ""}
            onChange={(e) =>
              onChange({
                ...filters,
                from_date: e.target.value
                  ? new Date(e.target.value).toISOString()
                  : undefined,
                page: 1,
              })
            }
            className="h-9 rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid)] px-3 py-1 text-xs text-[var(--ink-primary)] font-mono focus:border-[var(--color-blue)] focus:outline-none"
          />
        </div>

        <div className="flex items-center gap-2">
          <span className="text-[var(--ink-tertiary)] text-xs">To</span>
          <input
            type="date"
            value={filters.to_date ? filters.to_date.split("T")[0] : ""}
            onChange={(e) =>
              onChange({
                ...filters,
                to_date: e.target.value
                  ? new Date(e.target.value + "T23:59:59").toISOString()
                  : undefined,
                page: 1,
              })
            }
            className="h-9 rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid)] px-3 py-1 text-xs text-[var(--ink-primary)] font-mono focus:border-[var(--color-blue)] focus:outline-none"
          />
        </div>
      </div>
    </div>
  );
}
