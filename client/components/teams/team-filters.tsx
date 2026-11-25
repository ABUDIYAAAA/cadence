"use client";

import React from "react";
import { Search, X, RotateCcw } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { TeamFilter } from "@/types/teams";

interface TeamFiltersProps {
  filters: TeamFilter;
  onChange: (filters: TeamFilter) => void;
  onReset: () => void;
}

export function TeamFilters({ filters, onChange, onReset }: TeamFiltersProps) {
  const hasActiveFilters = Boolean(filters.search);

  return (
    <div className="space-y-3 font-sans">
      <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
        {/* Search */}
        <div className="relative flex-1 max-w-md">
          <Search className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--ink-tertiary)]" />
          <Input
            placeholder="Search teams by name..."
            value={filters.search || ""}
            onChange={(e) =>
              onChange({ ...filters, search: e.target.value, page: 1 })
            }
            className="h-9 pl-9 pr-8 text-xs"
          />
          {filters.search && (
            <button
              onClick={() => onChange({ ...filters, search: "", page: 1 })}
              aria-label="Clear search"
              className="absolute right-2.5 top-1/2 -translate-y-1/2 text-[var(--ink-tertiary)] hover:text-[var(--ink-primary)] cursor-pointer"
              type="button"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          )}
        </div>

        {/* Reset */}
        {hasActiveFilters && (
          <Button
            variant="outline"
            size="sm"
            onClick={onReset}
            className="h-9 text-xs gap-1.5 shrink-0"
          >
            <RotateCcw className="h-3.5 w-3.5" />
            <span>Reset Search</span>
          </Button>
        )}
      </div>
    </div>
  );
}
