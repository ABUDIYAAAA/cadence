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
  { value: "ticket.issued", label: "[Ticket] Pass Issued" },
  { value: "ticket.check_in", label: "[Ticket] Check-In Recorded" },
  { value: "ticket.duplicate_scan", label: "[Ticket] Duplicate Scan" },
  // Judging
  { value: "judging.round_create", label: "[Judging] Round Created" },
  { value: "judging.round_activate", label: "[Judging] Round Activated" },
  { value: "judging.slot_start", label: "[Judging] Slot Started" },
  { value: "judging.score_lock", label: "[Judging] Score Locked" },
  { value: "judging.slot_complete", label: "[Judging] Slot Completed" },
  // Notification
  { value: "notification.broadcast", label: "[Notification] Broadcast Sent" },
  { value: "notification.subscribe", label: "[Notification] Push Subscribed" },
  { value: "notification.unsubscribe", label: "[Notification] Push Unsubscribed" },
];

const TARGET_TYPE_OPTIONS = [
  { value: "", label: "All Target Types" },
  { value: "user", label: "User" },
  { value: "team", label: "Team" },
  { value: "ticket", label: "Ticket" },
  { value: "session", label: "Session" },
  { value: "auth", label: "Auth" },
  { value: "judging_round", label: "Judging Round" },
  { value: "judging_slot", label: "Judging Slot" },
  { value: "judging_panel", label: "Judging Panel" },
  { value: "notification", label: "Notification" },
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
    <div className="border border-neutral-300 bg-white p-4 font-mono space-y-3">
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
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-neutral-400 pointer-events-none" />
          {filters.search && (
            <button
              onClick={() => onChange({ ...filters, search: "", page: 1 })}
              className="absolute right-2.5 top-2.5 text-neutral-400 hover:text-black"
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
            <div className="hidden lg:flex items-center text-[10px] text-neutral-400 gap-1 pl-1">
              <Filter className="h-3 w-3" />
              <span>FILTER ACTIVE TRAIL</span>
            </div>
          )}
        </div>
      </div>

      {/* Date Range Sub-Bar */}
      <div className="flex flex-wrap items-center gap-3 pt-2 border-t border-neutral-100 text-xs">
        <span className="text-neutral-500 font-semibold text-[11px] uppercase">
          Timestamp Range:
        </span>
        <div className="flex items-center gap-2">
          <span className="text-neutral-400 text-[10px]">From</span>
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
            className="h-8 border border-neutral-300 bg-white px-2 py-1 text-xs text-black font-mono focus:border-black focus:outline-none"
          />
        </div>

        <div className="flex items-center gap-2">
          <span className="text-neutral-400 text-[10px]">To</span>
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
            className="h-8 border border-neutral-300 bg-white px-2 py-1 text-xs text-black font-mono focus:border-black focus:outline-none"
          />
        </div>
      </div>
    </div>
  );
}
