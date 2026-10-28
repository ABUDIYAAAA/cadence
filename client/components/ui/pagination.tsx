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
        "flex flex-col sm:flex-row items-center justify-between gap-4 border-t border-neutral-200 pt-4 font-mono text-xs",
        className
      )}
    >
      <div className="flex items-center gap-3 text-neutral-600">
        <span>
          Showing page <strong className="text-black">{page}</strong> of{" "}
          <strong className="text-black">{totalPages || 1}</strong>
        </span>
        {totalCount !== undefined && (
          <span className="text-neutral-400">
            ({totalCount} total {totalCount === 1 ? "record" : "records"})
          </span>
        )}
      </div>

      <div className="flex items-center gap-3">
        {onLimitChange && limit && (
          <div className="flex items-center gap-2">
            <span className="text-neutral-500">Rows:</span>
            <select
              value={limit}
              onChange={(e) => onLimitChange(Number(e.target.value))}
              className="h-8 border border-neutral-300 bg-white px-2 py-0.5 text-xs text-black focus:border-black focus:outline-none"
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

          <span className="px-2 py-1 font-bold text-black border border-black bg-neutral-100 min-w-7 text-center">
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
