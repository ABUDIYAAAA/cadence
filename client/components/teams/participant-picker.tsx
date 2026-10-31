"use client";

import React, { useEffect, useId, useRef, useState } from "react";
import { Search, X } from "lucide-react";
import { useUsers } from "@/hooks/use-users";
import { UserDetailResponse } from "@/types/users";
import { cn } from "@/lib/utils";

interface ParticipantPickerProps {
  value: UserDetailResponse | null;
  onChange: (user: UserDetailResponse | null) => void;
  disabled?: boolean;
  error?: string;
}

/** Type-to-search picker for participants who are not on a team yet. */
export function ParticipantPicker({ value, onChange, disabled, error }: ParticipantPickerProps) {
  const listId = useId();
  const rootRef = useRef<HTMLDivElement>(null);
  const [query, setQuery] = useState("");
  const [debounced, setDebounced] = useState("");
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);

  useEffect(() => {
    const t = setTimeout(() => setDebounced(query.trim()), 250);
    return () => clearTimeout(t);
  }, [query]);

  useEffect(() => {
    const onDown = (e: MouseEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, []);

  const { data, isFetching } = useUsers(
    { role: "participant", no_team: true, is_active: true, search: debounced, limit: 8 },
    { enabled: open && !value }
  );
  const results = data?.data ?? [];

  const select = (user: UserDetailResponse) => {
    onChange(user);
    setQuery("");
    setDebounced("");
    setOpen(false);
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setOpen(true);
      setActive((i) => Math.min(i + 1, Math.max(results.length - 1, 0)));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActive((i) => Math.max(i - 1, 0));
    } else if (e.key === "Enter" && open && results[active]) {
      e.preventDefault();
      select(results[active]);
    } else if (e.key === "Escape") {
      setOpen(false);
    }
  };

  if (value) {
    return (
      <div className="space-y-1">
        <div className="flex h-9 items-center justify-between gap-2 rounded-[14px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] px-3 text-xs">
          <span className="truncate text-[var(--ink-primary)]">
            <strong>{value.name}</strong> <span className="text-[var(--ink-secondary)]">({value.email})</span>
          </span>
          <button
            type="button"
            onClick={() => onChange(null)}
            disabled={disabled}
            aria-label="Clear selected participant"
            className="shrink-0 text-[var(--ink-secondary)] hover:text-[var(--ink-primary)]"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        </div>
        {error && <p className="text-xs text-[var(--color-coral)] font-medium">{error}</p>}
      </div>
    );
  }

  return (
    <div ref={rootRef} className="relative space-y-1">
      <div className="relative">
        <Search className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-[var(--ink-tertiary)]" />
        <input
          type="text"
          role="combobox"
          aria-expanded={open}
          aria-controls={listId}
          aria-autocomplete="list"
          value={query}
          disabled={disabled}
          placeholder="Search participants by name or email…"
          onChange={(e) => {
            setQuery(e.target.value);
            setActive(0);
            setOpen(true);
          }}
          onFocus={() => setOpen(true)}
          onKeyDown={onKeyDown}
          className={cn(
            "h-9 w-full rounded-[14px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] pl-9 pr-3 text-xs text-[var(--ink-primary)] placeholder:text-[var(--ink-tertiary)] focus:border-[var(--color-blue)] focus:outline-none focus:ring-3 focus:ring-[var(--color-blue)]/20 disabled:opacity-50",
            error && "border-[var(--color-coral)]"
          )}
        />
      </div>

      {open && (
        <ul
          id={listId}
          role="listbox"
          className="absolute z-20 mt-1 max-h-56 w-full overflow-y-auto rounded-[14px] border border-[var(--card-line)] bg-[var(--card-solid)] py-1 shadow-lg"
        >
          {results.length === 0 ? (
            <li className="px-3 py-2 text-xs text-[var(--ink-tertiary)]">
              {isFetching ? "Searching…" : "No participants without a team match."}
            </li>
          ) : (
            results.map((u, i) => (
              <li
                key={u.id}
                role="option"
                aria-selected={i === active}
                onMouseEnter={() => setActive(i)}
                onMouseDown={(e) => {
                  e.preventDefault();
                  select(u);
                }}
                className={cn(
                  "cursor-pointer px-3 py-1.5 text-xs",
                  i === active && "bg-[var(--card-solid-subtle)]"
                )}
              >
                <div className="font-medium text-[var(--ink-primary)] truncate">{u.name}</div>
                <div className="text-[11px] text-[var(--ink-secondary)] truncate">{u.email}</div>
              </li>
            ))
          )}
        </ul>
      )}
      {error && <p className="text-xs text-[var(--color-coral)] font-medium">{error}</p>}
    </div>
  );
}
