"use client";

import React from "react";
import { Search, X, RotateCcw, Filter } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import {
  AUDIT_ACTION_OPTIONS,
  AUDIT_TARGET_TYPE_OPTIONS,
  hasActiveAuditFilters,
} from "@/lib/audit-registry";
import type { Competition } from "@/types/competitions";
import { AuditLogFilter } from "@/types/audit";

interface AuditFiltersProps {
  filters: AuditLogFilter;
  competitions?: ReadonlyArray<Competition>;
  onChange: (filters: AuditLogFilter) => void;
  onReset: () => void;
}

export function AuditFilters({
  filters,
  competitions = [],
  onChange,
  onReset,
}: AuditFiltersProps) {
  const hasActiveFilters = hasActiveAuditFilters(filters);
  const showCompetitionFilter = competitions.length > 0;

  return (
    <div className="space-y-3 font-sans">
      <div
        className={`grid grid-cols-1 sm:grid-cols-2 ${
          showCompetitionFilter ? "lg:grid-cols-6" : "lg:grid-cols-5"
        } gap-3`}
      >
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
              aria-label="Clear search"
            >
              <X className="h-4 w-4" />
            </button>
          )}
        </div>

        {/* Competition Dropdown (when inside an event with competitions) */}
        {showCompetitionFilter && (
          <div>
            <Select
              aria-label="Filter by competition"
              value={filters.competition_id || ""}
              onChange={(e) =>
                onChange({
                  ...filters,
                  competition_id: e.target.value || undefined,
                  page: 1,
                })
              }
              className="text-xs"
            >
              <option value="">All Competitions</option>
              {competitions.map((comp) => (
                <option key={comp.id} value={comp.id}>
                  {comp.name}
                </option>
              ))}
            </Select>
          </div>
        )}

        {/* Action Dropdown */}
        <div>
          <Select
            aria-label="Filter by action"
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
            {AUDIT_ACTION_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </Select>
        </div>

        {/* Target Type Dropdown */}
        <div>
          <Select
            aria-label="Filter by target type"
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
            {AUDIT_TARGET_TYPE_OPTIONS.map((opt) => (
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
