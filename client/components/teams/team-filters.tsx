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
    <div className="border border-neutral-300 bg-white p-4 font-mono space-y-3">
      <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
        {/* Search */}
        <div className="relative flex-1 max-w-md">
          <Input
            placeholder="Search teams by name..."
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
