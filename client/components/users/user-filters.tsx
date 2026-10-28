"use client";

import React from "react";
import { Search, X, RotateCcw } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import { UserFilter } from "@/types/users";

interface UserFiltersProps {
  filters: UserFilter;
  onChange: (filters: UserFilter) => void;
  onReset: () => void;
}

export function UserFilters({ filters, onChange, onReset }: UserFiltersProps) {
  const hasActiveFilters = Boolean(
    filters.search || filters.role || filters.is_active !== undefined
  );

  return (
    <div className="border border-neutral-300 bg-white p-4 font-mono space-y-3">
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
        {/* Search */}
        <div className="relative">
          <Input
            placeholder="Search by name or email..."
            value={filters.search || ""}
            onChange={(e) => onChange({ ...filters, search: e.target.value, page: 1 })}
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

        {/* Role Filter */}
        <div>
          <Select
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
            <option value="admin">Admin</option>
            <option value="judge">Judge</option>
            <option value="volunteer">Volunteer</option>
            <option value="participant">Participant</option>
          </Select>
        </div>

        {/* Status Filter */}
        <div>
          <Select
            value={
              filters.is_active === undefined
                ? ""
                : filters.is_active
                ? "active"
                : "inactive"
            }
            onChange={(e) => {
              const val = e.target.value;
              onChange({
                ...filters,
                is_active:
                  val === "active" ? true : val === "inactive" ? false : undefined,
                page: 1,
              });
            }}
            className="text-xs"
          >
            <option value="">All Statuses</option>
            <option value="active">Active Only</option>
            <option value="inactive">Inactive Only</option>
          </Select>
        </div>

        {/* Reset Action */}
        <div className="flex items-center gap-2">
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
