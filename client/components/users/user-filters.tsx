"use client";

import React from "react";
import { Search, X, RotateCcw } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import {
  encodeTeamAssignmentFilter,
  parseTeamAssignmentFilter,
} from "@/lib/competition-utils";
import type { Competition } from "@/types/competitions";
import { UserFilter } from "@/types/users";

interface UserFiltersProps {
  filters: UserFilter;
  mode?: "event" | "platform";
  /** Competitions in the active event, used to filter members without a team in a competition. */
  competitions?: ReadonlyArray<Competition>;
  /** Platform devs can also browse soft-deleted users. */
  canViewRemoved?: boolean;
  onChange: (filters: UserFilter) => void;
  onReset: () => void;
}

export function UserFilters({
  filters,
  mode = "event",
  competitions = [],
  canViewRemoved = false,
  onChange,
  onReset,
}: UserFiltersProps) {
  const hasActiveFilters = Boolean(
    filters.search ||
      filters.role ||
      filters.no_team ||
      filters.competition_id ||
      filters.is_active !== undefined ||
      filters.is_banned !== undefined ||
      filters.deleted
  );

  const statusValue =
    mode === "event"
      ? filters.is_banned === undefined
        ? ""
        : filters.is_banned
        ? "banned"
        : "active"
      : filters.deleted
      ? "removed"
      : filters.is_active === undefined
      ? ""
      : filters.is_active
      ? "active"
      : "inactive";

  const teamAssignmentValue = encodeTeamAssignmentFilter(filters);

  return (
    <div className="space-y-3 font-sans">
      <div
        className={`grid grid-cols-1 sm:grid-cols-2 ${
          mode === "event" ? "lg:grid-cols-5" : "lg:grid-cols-4"
        } gap-3`}
      >
        {/* Search */}
        <div className="relative">
          <Input
            placeholder="Search by name or email..."
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
              aria-label="Clear search"
            >
              <X className="h-4 w-4" />
            </button>
          )}
        </div>

        {/* Role Filter */}
        <div>
          <Select
            aria-label="Filter by role"
            value={filters.role || ""}
            onChange={(e) =>
              onChange({
                ...filters,
                role: e.target.value || undefined,
                page: 1,
              })
            }
            className="text-xs"
          >
            <option value="">All Roles</option>
            {mode === "platform" ? (
              <>
                <option value="dev">Dev</option>
                <option value="user">User</option>
              </>
            ) : (
              <>
                <option value="owner">Owner</option>
                <option value="admin">Admin</option>
                <option value="judge">Judge</option>
                <option value="volunteer">Volunteer</option>
                <option value="participant">Participant</option>
              </>
            )}
          </Select>
        </div>

        {/* Competition Team Assignment Filter (Event mode only) */}
        {mode === "event" && (
          <div>
            <Select
              aria-label="Filter by team assignment"
              value={teamAssignmentValue}
              onChange={(e) => {
                const parsed = parseTeamAssignmentFilter(e.target.value);
                onChange({
                  ...filters,
                  no_team: parsed.no_team,
                  competition_id: parsed.competition_id,
                  page: 1,
                });
              }}
              className="text-xs"
            >
              <option value="">All team states</option>
              <option value="unassigned:any">
                No team (any competition)
              </option>
              {competitions.map((comp) => (
                <option key={`unassigned:${comp.id}`} value={`unassigned:${comp.id}`}>
                  No team in {comp.name}
                </option>
              ))}
              {competitions.length > 1 &&
                competitions.map((comp) => (
                  <option key={`competition:${comp.id}`} value={`competition:${comp.id}`}>
                    Show team in {comp.name}
                  </option>
                ))}
            </Select>
          </div>
        )}

        {/* Status Filter */}
        <div>
          <Select
            aria-label="Filter by status"
            value={statusValue}
            onChange={(e) => {
              const val = e.target.value;
              if (mode === "event") {
                onChange({
                  ...filters,
                  is_banned:
                    val === "banned"
                      ? true
                      : val === "active"
                      ? false
                      : undefined,
                  is_active: undefined,
                  deleted: undefined,
                  page: 1,
                });
              } else {
                onChange({
                  ...filters,
                  deleted: val === "removed" ? true : undefined,
                  is_active:
                    val === "active"
                      ? true
                      : val === "inactive"
                      ? false
                      : undefined,
                  is_banned: undefined,
                  page: 1,
                });
              }
            }}
            className="text-xs"
          >
            <option value="">All Statuses</option>
            <option value="active">Active Only</option>
            {mode === "event" ? (
              <option value="banned">Banned in Event</option>
            ) : (
              <>
                <option value="inactive">Blacklisted / Inactive</option>
                {canViewRemoved && <option value="removed">Removed</option>}
              </>
            )}
          </Select>
        </div>

        {/* Reset Action */}
        <div className="flex items-center">
          {hasActiveFilters && (
            <Button
              variant="outline"
              size="sm"
              onClick={onReset}
              className="h-9 w-full sm:w-auto text-xs gap-1.5"
            >
              <RotateCcw className="h-3.5 w-3.5" />
              Reset Filters
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}
