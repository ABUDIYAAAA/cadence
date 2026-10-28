import React from "react";
import { Button } from "./button";
import { cn } from "@/lib/utils";

export interface PaginationProps {
  page: number;
  totalPages: number;
  totalCount?: number;
  limit?: number;
  onPageChange: (newPage: number) => void;
  onLimitChange?: (newLimit: number) => void;
  className?: string;
}

export function Pagination({
  page,
  totalPages,
  totalCount,
  limit,
  onPageChange,
  onLimitChange,
  className,
}: PaginationProps) {
  const canPrev = page > 1;
  const canNext = page < totalPages;

  return (
    <div
      className={cn(
        "flex flex-col sm:flex-row items-center justify-between gap-4 border-t border-[var(--card-line)] pt-4 font-sans text-xs",
        className
      )}
    >
      <div className="flex items-center gap-2 text-[var(--ink-secondary)]">
        <span>
          Showing page <strong className="text-[var(--ink-primary)] font-bold">{page}</strong> of{" "}
          <strong className="text-[var(--ink-primary)] font-bold">{totalPages || 1}</strong>
        </span>
        {totalCount !== undefined && (
          <span className="text-[var(--ink-tertiary)]">
            ({totalCount} total {totalCount === 1 ? "record" : "records"})
          </span>
        )}
      </div>

      <div className="flex items-center gap-3">
        {onLimitChange && limit && (
          <div className="flex items-center gap-1.5 text-xs text-[var(--ink-secondary)]">
            <span>Rows:</span>
            <select
              value={limit}
              onChange={(e) => onLimitChange(Number(e.target.value))}
              className="h-8 rounded-[10px] border border-[var(--card-line)] bg-[var(--card-solid)] px-2 text-xs text-[var(--ink-primary)] focus:border-[var(--color-blue)] focus:outline-none cursor-pointer"
            >
              <option value={10}>10</option>
              <option value={20}>20</option>
              <option value={50}>50</option>
              <option value={100}>100</option>
            </select>
          </div>
        )}

        <div className="flex items-center gap-1">
          <Button
            variant="outline"
            size="sm"
            onClick={() => onPageChange(1)}
            disabled={!canPrev}
            title="First page"
            className="h-8 px-2 text-xs"
          >
            «
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => onPageChange(page - 1)}
            disabled={!canPrev}
            className="h-8 px-2.5 text-xs"
          >
            ‹ Prev
          </Button>

          <span className="px-3 py-1 font-bold text-[var(--ink-primary)] rounded-[10px] bg-[var(--card-solid-subtle)] border border-[var(--card-line)] min-w-8 text-center text-xs">
            {page}
          </span>

          <Button
            variant="outline"
            size="sm"
            onClick={() => onPageChange(page + 1)}
            disabled={!canNext}
            className="h-8 px-2.5 text-xs"
          >
            Next ›
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => onPageChange(totalPages)}
            disabled={!canNext}
            title="Last page"
            className="h-8 px-2 text-xs"
          >
            »
          </Button>
        </div>
      </div>
    </div>
  );
}
