"use client";

import React, { useCallback, useEffect, useId, useMemo, useRef, useState } from "react";
import { Loader2, Search, X } from "lucide-react";
import { cn } from "@/lib/utils";

export interface SearchSelectOption {
  value: string;
  label: string;
  description?: string;
  meta?: string;
  disabled?: boolean;
}

export interface SearchSelectProps {
  query: string;
  onQueryChange: (query: string) => void;
  options: SearchSelectOption[];
  /** Currently selected option (for single-select mode). Omit for add/action mode. */
  selectedOption?: SearchSelectOption | null;
  onSelect: (option: SearchSelectOption) => void;
  onClear?: () => void;
  placeholder?: string;
  emptyMessage?: string;
  loadingMessage?: string;
  isLoading?: boolean;
  disabled?: boolean;
  /** Maximum number of visible matches. Defaults to 3. */
  maxVisible?: number;
  /** Optional total match count from the server when larger than options.length. */
  totalMatches?: number;
  error?: string;
  helperText?: string;
  ariaLabel?: string;
  size?: "sm" | "md";
  onOpenChange?: (open: boolean) => void;
}

export function SearchSelect({
  query,
  onQueryChange,
  options,
  selectedOption,
  onSelect,
  onClear,
  placeholder = "Search...",
  emptyMessage = "No matches found",
  loadingMessage = "Searching...",
  isLoading = false,
  disabled = false,
  maxVisible = 3,
  totalMatches,
  error,
  helperText,
  ariaLabel,
  size = "md",
  onOpenChange,
}: SearchSelectProps) {
  const listId = useId();
  const rootRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const [open, setOpen] = useState(false);
  const [highlightedIndex, setHighlightedIndex] = useState(0);
  const onOpenChangeRef = useRef(onOpenChange);
  useEffect(() => {
    onOpenChangeRef.current = onOpenChange;
  }, [onOpenChange]);

  const visibleOptions = useMemo(
    () => options.slice(0, maxVisible),
    [options, maxVisible]
  );

  const activeIndex =
    visibleOptions.length > 0
      ? Math.min(highlightedIndex, visibleOptions.length - 1)
      : 0;

  const effectiveTotal = Math.max(totalMatches ?? options.length, options.length);
  const hasMore = effectiveTotal > visibleOptions.length;

  const setOpenState = useCallback((next: boolean) => {
    setOpen(next);
    onOpenChangeRef.current?.(next);
  }, []);

  useEffect(() => {
    if (!open) return;
    const handleMouseDown = (e: MouseEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) {
        setOpenState(false);
      }
    };
    document.addEventListener("mousedown", handleMouseDown);
    return () => document.removeEventListener("mousedown", handleMouseDown);
  }, [open, setOpenState]);

  const handleSelect = (option: SearchSelectOption) => {
    if (option.disabled) return;
    onSelect(option);
    setOpenState(false);
  };

  const handleClearSelection = () => {
    onClear?.();
    onQueryChange("");
    setTimeout(() => {
      inputRef.current?.focus();
    }, 0);
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      if (!open) {
        setOpenState(true);
      } else if (visibleOptions.length > 0) {
        setHighlightedIndex((i) => Math.min(i + 1, visibleOptions.length - 1));
      }
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      if (open && visibleOptions.length > 0) {
        setHighlightedIndex((i) => Math.max(i - 1, 0));
      }
    } else if (e.key === "Enter" && open) {
      const target = visibleOptions[activeIndex];
      if (target && !target.disabled) {
        e.preventDefault();
        handleSelect(target);
      }
    } else if (e.key === "Escape" && open) {
      e.preventDefault();
      e.stopPropagation();
      e.nativeEvent.stopImmediatePropagation();
      setOpenState(false);
    } else if (e.key === "Tab" && open) {
      setOpenState(false);
    }
  };

  const rowHeight = size === "sm" ? "h-9" : "h-[42px]";

  if (selectedOption) {
    return (
      <div className="w-full space-y-1.5 text-left font-sans">
        <div
          className={cn(
            "flex w-full items-center justify-between gap-2 rounded-[14px] border border-[var(--card-line)] bg-[var(--card-solid)] px-3.5 text-xs",
            rowHeight,
            error && "border-[var(--color-coral)] ring-3 ring-[var(--color-coral)]/20",
            disabled && "opacity-50"
          )}
        >
          <div className="flex min-w-0 items-center gap-2 truncate">
            <span className="truncate font-medium text-[var(--ink-primary)]">
              {selectedOption.label}
            </span>
            {selectedOption.description && (
              <span className="truncate text-[11px] text-[var(--ink-secondary)]">
                {selectedOption.description}
              </span>
            )}
          </div>
          <div className="flex shrink-0 items-center gap-2">
            {selectedOption.meta && (
              <span className="text-[10px] capitalize text-[var(--ink-tertiary)]">
                {selectedOption.meta}
              </span>
            )}
            {onClear && (
              <button
                type="button"
                onClick={handleClearSelection}
                disabled={disabled}
                aria-label="Clear selection"
                className="rounded-full p-0.5 text-[var(--ink-secondary)] transition-colors hover:bg-[var(--card-solid-subtle)] hover:text-[var(--ink-primary)] cursor-pointer disabled:cursor-not-allowed"
              >
                <X className="h-3.5 w-3.5" />
              </button>
            )}
          </div>
        </div>
        {error && (
          <p role="alert" className="text-xs font-medium text-[var(--color-coral)]">
            {error}
          </p>
        )}
        {helperText && !error && (
          <p className="text-xs text-[var(--ink-secondary)]">{helperText}</p>
        )}
      </div>
    );
  }

  return (
    <div ref={rootRef} className="w-full space-y-1.5 text-left font-sans">
      <div className={cn("relative", rowHeight)}>
        <div
          className={cn(
            "w-full overflow-hidden rounded-[14px] border bg-[var(--card-solid)] transition-[border-color,box-shadow] duration-150 ease-out",
            open
              ? "absolute inset-x-0 top-0 z-30 border-[var(--color-blue)] shadow-[var(--shadow-container)] ring-3 ring-[var(--color-blue)]/20"
              : "border-[var(--card-line)]",
            error && !open && "border-[var(--color-coral)] ring-3 ring-[var(--color-coral)]/20",
            error && open && "border-[var(--color-coral)] ring-3 ring-[var(--color-coral)]/25",
            disabled && "opacity-50"
          )}
        >
          {/* Search input row */}
          <div className={cn("flex items-center gap-2.5 px-3.5", rowHeight)}>
            <Search className="h-3.5 w-3.5 shrink-0 text-[var(--ink-tertiary)]" />
            <input
              ref={inputRef}
              type="text"
              role="combobox"
              aria-expanded={open}
              aria-controls={open ? listId : undefined}
              aria-autocomplete="list"
              aria-label={ariaLabel}
              aria-invalid={Boolean(error) || undefined}
              value={query}
              disabled={disabled}
              placeholder={placeholder}
              onFocus={() => setOpenState(true)}
              onChange={(e) => {
                onQueryChange(e.target.value);
                setHighlightedIndex(0);
                if (!open) setOpenState(true);
              }}
              onKeyDown={handleKeyDown}
              className="h-full w-full bg-transparent text-xs text-[var(--ink-primary)] placeholder:text-[var(--ink-tertiary)] focus:outline-none disabled:cursor-not-allowed"
            />
            {isLoading ? (
              <Loader2 className="h-3.5 w-3.5 shrink-0 animate-spin text-[var(--ink-tertiary)]" />
            ) : query ? (
              <button
                type="button"
                onMouseDown={(e) => e.preventDefault()}
                onClick={() => {
                  onQueryChange("");
                  inputRef.current?.focus();
                }}
                aria-label="Clear search"
                className="shrink-0 rounded-full p-0.5 text-[var(--ink-tertiary)] transition-colors hover:bg-[var(--card-solid-subtle)] hover:text-[var(--ink-primary)] cursor-pointer"
              >
                <X className="h-3.5 w-3.5" />
              </button>
            ) : null}
          </div>

          {/* Fused search results */}
          {open && (
            <div className="border-t border-[var(--card-line)]">
              <ul id={listId} role="listbox" aria-label={ariaLabel} className="p-1">
                {visibleOptions.length === 0 ? (
                  <li className="px-2.5 py-2 text-xs text-[var(--ink-tertiary)]">
                    {isLoading ? loadingMessage : emptyMessage}
                  </li>
                ) : (
                  visibleOptions.map((opt, idx) => {
                    const isHighlighted = idx === activeIndex;
                    return (
                      <li
                        key={opt.value}
                        role="option"
                        aria-selected={isHighlighted}
                        aria-disabled={opt.disabled || undefined}
                        onMouseEnter={() => setHighlightedIndex(idx)}
                        onMouseDown={(e) => e.preventDefault()}
                        onClick={() => handleSelect(opt)}
                        className={cn(
                          "flex items-center justify-between gap-2 rounded-[8px] px-2.5 py-2 text-xs transition-colors",
                          opt.disabled
                            ? "cursor-not-allowed opacity-40"
                            : "cursor-pointer",
                          isHighlighted
                            ? "bg-[var(--card-solid-subtle)] text-[var(--ink-primary)]"
                            : "text-[var(--ink-primary)] hover:bg-[var(--card-solid-subtle)]"
                        )}
                      >
                        <div className="min-w-0 flex-1">
                          <div className="truncate font-medium text-[var(--ink-primary)]">
                            {opt.label}
                          </div>
                          {opt.description && (
                            <div className="truncate text-[11px] text-[var(--ink-secondary)]">
                              {opt.description}
                            </div>
                          )}
                        </div>
                        {opt.meta && (
                          <span className="shrink-0 text-[10px] capitalize text-[var(--ink-tertiary)]">
                            {opt.meta}
                          </span>
                        )}
                      </li>
                    );
                  })
                )}
              </ul>
              {hasMore && (
                <div className="border-t border-[var(--card-line)] bg-[var(--card-solid-subtle)]/50 px-3.5 py-1.5 text-[11px] text-[var(--ink-tertiary)]">
                  Showing top {visibleOptions.length} of {effectiveTotal} matches — keep typing to narrow
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      {error && (
        <p role="alert" className="text-xs font-medium text-[var(--color-coral)]">
          {error}
        </p>
      )}
      {helperText && !error && (
        <p className="text-xs text-[var(--ink-secondary)]">{helperText}</p>
      )}
    </div>
  );
}
