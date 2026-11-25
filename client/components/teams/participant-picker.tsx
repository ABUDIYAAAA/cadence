"use client";

import React, { useEffect, useState } from "react";
import { useUsers } from "@/hooks/use-users";
import { useTenant } from "@/providers/tenant-provider";
import { UserDetailResponse } from "@/types/users";
import { SearchSelect } from "@/components/ui/search-select";

interface ParticipantPickerProps {
  value: UserDetailResponse | null;
  onChange: (user: UserDetailResponse | null) => void;
  disabled?: boolean;
  error?: string;
}

/** Type-to-search picker for participants who are not on a team yet. */
export function ParticipantPicker({ value, onChange, disabled, error }: ParticipantPickerProps) {
  const { activeCompetition } = useTenant();
  const [query, setQuery] = useState("");
  const [debounced, setDebounced] = useState("");
  const [open, setOpen] = useState(false);

  useEffect(() => {
    const t = setTimeout(() => setDebounced(query.trim()), 250);
    return () => clearTimeout(t);
  }, [query]);

  const { data, isFetching } = useUsers(
    {
      role: "participant",
      no_team: true,
      competition_id: activeCompetition?.id,
      is_active: true,
      search: debounced,
      limit: 8,
    },
    { enabled: open && !value }
  );
  const results = data?.data ?? [];

  return (
    <SearchSelect
      ariaLabel="Search participants"
      placeholder="Search participants by name or email..."
      size="sm"
      query={query}
      onQueryChange={setQuery}
      onOpenChange={setOpen}
      isLoading={isFetching}
      loadingMessage="Searching..."
      emptyMessage="No participants without a team match."
      disabled={disabled}
      error={error}
      maxVisible={3}
      totalMatches={data?.total_count}
      selectedOption={
        value
          ? {
              value: value.id,
              label: value.name,
              description: value.email,
            }
          : null
      }
      options={results.map((u) => ({
        value: u.id,
        label: u.name,
        description: u.email,
      }))}
      onSelect={(opt) => {
        const picked = results.find((u) => u.id === opt.value) ?? null;
        if (picked) {
          onChange(picked);
          setQuery("");
          setDebounced("");
        }
      }}
      onClear={() => onChange(null)}
    />
  );
}
