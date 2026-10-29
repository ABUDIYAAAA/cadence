"use client";

import React, { useState, useMemo } from "react";
import {
  Search,
  Clock,
  Play,
  ExternalLink,
  Shield,
  Volume2,
} from "lucide-react";
import { useVolunteerSchedule } from "@/hooks/use-judging";
import { VolunteerPanelSchedule } from "@/types/judging";
import { SectionToolbar, MetricGrid, Metric } from "./section-shell";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { SkeletonCard } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";

export function VolunteerScheduleView() {
  const { data: rawSchedule, isLoading, isFetching, refetch } = useVolunteerSchedule();
  const schedule = useMemo(() => (Array.isArray(rawSchedule) ? rawSchedule : []), [rawSchedule]);
  const [searchQuery, setSearchQuery] = useState("");
  const [selectedRoom, setSelectedRoom] = useState<string>("all");

  const roomsList = useMemo(() => {
    const set = new Set<string>();
    schedule.forEach((p) => {
      if (p.room) set.add(p.room);
    });
    return Array.from(set);
  }, [schedule]);

  const filteredSchedule = useMemo(() => {
    return schedule.filter((panel) => {
      const matchesRoom = selectedRoom === "all" || panel.room === selectedRoom;

      if (!matchesRoom) return false;
      if (!searchQuery.trim()) return true;

      const q = searchQuery.toLowerCase();
      const inPanel =
        panel.panel_name.toLowerCase().includes(q) ||
        panel.room.toLowerCase().includes(q) ||
        panel.round_name.toLowerCase().includes(q);

      const inActive =
        panel.active_team?.team_name.toLowerCase().includes(q);

      const inQueued = panel.queued_teams.some((t) =>
        t.team_name.toLowerCase().includes(q)
      );

      const inUpcoming = panel.upcoming_teams.some((t) =>
        t.team_name.toLowerCase().includes(q)
      );

      return inPanel || inActive || inQueued || inUpcoming;
    });
  }, [schedule, selectedRoom, searchQuery]);

  const stats = useMemo(() => {
    const totalRooms = schedule.length;
    let activeRooms = 0;
    let totalCompleted = 0;
    let totalTeams = 0;
    let onDeckCount = 0;

    schedule.forEach((p) => {
      if (p.active_team) activeRooms++;
      totalCompleted += p.completed_count || 0;
      totalTeams += p.total_count || 0;
      onDeckCount += (p.queued_teams || []).length;
    });

    return { totalRooms, activeRooms, totalCompleted, totalTeams, onDeckCount };
  }, [schedule]);

  const formatTime = (iso?: string | null) => {
    if (!iso) return "TBD";
    try {
      const date = new Date(iso);
      return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
    } catch {
      return "TBD";
    }
  };

  return (
    <div className="font-sans space-y-6">
      <SectionToolbar
        title="Room staging"
        subtitle="Live queues and on-deck teams for each room"
        onRefresh={() => refetch()}
        isFetching={isFetching}
      />

      <MetricGrid>
        <Metric label="Active rooms" value={stats.activeRooms} suffix={`/ ${stats.totalRooms}`} />
        <Metric label="Teams on deck" value={stats.onDeckCount} valueClassName="text-[var(--color-coral)]" />
        <Metric label="Evaluations done" value={stats.totalCompleted} suffix={`/ ${stats.totalTeams}`} />
        <Metric
          label="Overall progress"
          value={stats.totalTeams > 0 ? `${Math.round((stats.totalCompleted / stats.totalTeams) * 100)}%` : "0%"}
          valueClassName="text-[var(--color-blue)]"
        />
      </MetricGrid>

      {/* Primary Surface: Search/Filters + Rooms */}
      <div className="bg-[var(--card-solid)] border border-[var(--card-line)] rounded-[16px] p-4 sm:p-5 space-y-4">
        <div className="flex flex-col md:flex-row items-stretch md:items-center justify-between gap-3">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--ink-tertiary)]" />
            <Input
              placeholder="Search teams, rooms, or panels..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="pl-9 text-xs"
            />
          </div>

          {roomsList.length > 0 && (
            <div className="flex items-center gap-1.5 overflow-x-auto">
              {["all", ...roomsList].map((room) => (
                <Button
                  key={room}
                  variant={selectedRoom === room ? "primary" : "outline"}
                  size="sm"
                  onClick={() => setSelectedRoom(room)}
                  className="text-xs whitespace-nowrap rounded-full h-8 px-3"
                >
                  {room === "all" ? "All rooms" : room}
                </Button>
              ))}
            </div>
          )}
        </div>

        {isLoading ? (
          <div className="space-y-4">
            <SkeletonCard />
            <SkeletonCard />
          </div>
        ) : filteredSchedule.length === 0 ? (
          <EmptyState
            title="No active judging schedules"
            description="Once an administrator activates a judging round, room staging queues will populate automatically."
          />
        ) : (
          <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
            {filteredSchedule.map((panel) => (
              <RoomScheduleCard key={panel.panel_id} panel={panel} formatTime={formatTime} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function RoomScheduleCard({
  panel,
  formatTime,
}: {
  panel: VolunteerPanelSchedule;
  formatTime: (iso?: string | null) => string;
}) {
  const percentComplete =
    panel.total_count > 0
      ? Math.round((panel.completed_count / panel.total_count) * 100)
      : 0;

  return (
    <div className="rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid)] flex flex-col justify-between overflow-hidden">
      {/* Card Header */}
      <div className="border-b border-[var(--card-line)] p-5 bg-[var(--card-solid-subtle)]">
        <div className="flex items-start justify-between gap-2">
          <div>
            <div className="flex items-center gap-2">
              <span className="font-bold text-base text-[var(--ink-primary)]">{panel.room || "Room Unassigned"}</span>
              <Badge variant="blue" className="text-[10px] uppercase font-semibold">
                {panel.panel_name}
              </Badge>
            </div>
            <div className="text-xs text-[var(--ink-secondary)] mt-1 truncate">
              Round: <strong className="text-[var(--ink-primary)]">{panel.round_name}</strong>
            </div>
          </div>

          <div className="text-right">
            <span className="text-xs font-bold font-mono text-[var(--ink-primary)]">
              {panel.completed_count} / {panel.total_count} Evaluated
            </span>
            <div className="w-24 bg-[var(--card-line)] h-2 rounded-full mt-1.5 ml-auto overflow-hidden">
              <div
                className="bg-[#83A2DB] h-full rounded-full transition-all duration-300"
                style={{ width: `${percentComplete}%` }}
              />
            </div>
          </div>
        </div>

        <div className="flex items-center gap-3 mt-3 text-xs text-[var(--ink-secondary)]">
          <span className="flex items-center gap-1 font-mono">
            <Clock className="h-3 w-3 text-[var(--ink-tertiary)]" />
            {panel.slot_duration_mins}m per slot
          </span>
          <span>•</span>
          <span>Buffer: {panel.buffer_size} on-deck</span>
        </div>
      </div>

      {/* Card Body */}
      <div className="p-5 space-y-4 flex-1">
        {/* 1. CURRENTLY PRESENTING TEAM */}
        <div>
          <div className="text-xs font-semibold text-[var(--ink-tertiary)] mb-1.5 flex items-center gap-1.5">
            <Play className="h-3 w-3 fill-[#83A2DB] text-[var(--color-blue)]" />
            Currently In Room (Presenting)
          </div>

          {panel.active_team ? (
            <div className="rounded-[16px] border border-[#83A2DB]/30 bg-[var(--color-blue-dim)] p-3.5 flex items-start justify-between gap-3">
              <div>
                <div className="flex items-center gap-2">
                  <span className="font-bold text-sm text-[var(--ink-primary)]">
                    {panel.active_team.team_name}
                  </span>
                  <Badge variant="blue" className="text-[9px] uppercase font-semibold">
                    Slot #{panel.active_team.sequence_order}
                  </Badge>
                </div>
                <div className="text-xs text-[var(--ink-secondary)] mt-1 flex items-center gap-1 font-mono">
                  <Clock className="h-3 w-3 text-[var(--ink-tertiary)]" />
                  Started at {formatTime(panel.active_team.actual_start_time)}
                </div>
              </div>

              {panel.active_team.submission_links &&
                Object.keys(panel.active_team.submission_links).length > 0 && (
                  <div className="flex items-center gap-1">
                    {Object.entries(panel.active_team.submission_links).map(([k, url]) => (
                      <a
                        key={k}
                        href={url}
                        target="_blank"
                        rel="noreferrer"
                        title={k}
                        className="p-1 px-2 rounded-lg border border-[#83A2DB]/30 text-xs bg-[var(--card-solid)] hover:text-[var(--color-blue)] transition-colors inline-flex items-center gap-1"
                      >
                        <span className="capitalize text-[10px] font-bold">{k}</span>
                        <ExternalLink className="h-2.5 w-2.5 text-[var(--ink-tertiary)]" />
                      </a>
                    ))}
                  </div>
                )}
            </div>
          ) : (
            <div className="rounded-[14px] border border-dashed border-[var(--card-line)] p-3.5 text-center text-xs text-[var(--ink-tertiary)]">
              No team currently presenting in this room.
            </div>
          )}
        </div>

        {/* 2. ON-DECK QUEUED TEAMS (BUFFER TO ESCORT) */}
        <div>
          <div className="text-xs font-semibold text-[var(--color-coral)] mb-1.5 flex items-center justify-between">
            <span className="flex items-center gap-1.5">
              <Volume2 className="h-3.5 w-3.5" />
              On-Deck Staging Buffer ({panel.queued_teams.length} Call-Ups)
            </span>
            <span className="text-[9px] text-[var(--ink-tertiary)]">Escort to {panel.room}</span>
          </div>

          {panel.queued_teams.length === 0 ? (
            <div className="text-xs text-[var(--ink-tertiary)] italic p-3 rounded-[14px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)]">
              Queue buffer is clear.
            </div>
          ) : (
            <div className="space-y-1.5">
              {panel.queued_teams.map((t, idx) => (
                <div
                  key={t.slot_id}
                  className="rounded-[14px] border border-[#CE6969]/25 bg-[var(--color-coral-dim)] p-2.5 px-3 flex items-center justify-between text-xs"
                >
                  <div className="flex items-center gap-2.5">
                    <span className="w-5 h-5 rounded-full bg-[#CE6969] text-white text-[10px] font-bold flex items-center justify-center shrink-0">
                      {idx + 1}
                    </span>
                    <div>
                      <span className="font-bold text-[var(--ink-primary)]">{t.team_name}</span>
                      <span className="text-[var(--ink-tertiary)] text-[10px] ml-1.5 font-mono">
                        (Slot #{t.sequence_order})
                      </span>
                    </div>
                  </div>

                  <div className="text-right text-xs font-mono text-[var(--ink-secondary)]">
                    Est. {formatTime(t.estimated_start_time)}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* 3. UPCOMING SCHEDULE */}
        {panel.upcoming_teams.length > 0 && (
          <div>
            <div className="text-xs font-semibold text-[var(--ink-tertiary)] mb-1.5">
              Later in Schedule ({panel.upcoming_teams.length} Teams)
            </div>

            <div className="rounded-[14px] border border-[var(--card-line)] divide-y divide-[var(--card-line)] max-h-32 overflow-y-auto">
              {panel.upcoming_teams.map((t) => (
                <div
                  key={t.slot_id}
                  className="p-2 px-3 flex items-center justify-between text-xs bg-[var(--card-solid)] text-[var(--ink-secondary)]"
                >
                  <div className="flex items-center gap-2 truncate">
                    <span className="text-[var(--ink-tertiary)] text-[10px] font-mono">
                      #{t.sequence_order}
                    </span>
                    <span className="truncate text-[var(--ink-primary)] font-medium">{t.team_name}</span>
                  </div>
                  <span className="text-[10px] text-[var(--ink-tertiary)] font-mono shrink-0">
                    {formatTime(t.estimated_start_time)}
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Card Footer */}
      <div className="border-t border-[var(--card-line)] p-3 px-5 bg-[var(--card-solid-subtle)] text-xs text-[var(--ink-tertiary)] flex items-center justify-between">
        <span className="flex items-center gap-1.5">
          <Shield className="h-3 w-3 text-[var(--color-blue)]" />
          Live queue updates every 10s
        </span>
        <span className="uppercase font-semibold text-[var(--ink-secondary)] font-mono text-[10px]">
          Panel ID: {panel.panel_id.substring(0, 8)}...
        </span>
      </div>
    </div>
  );
}
