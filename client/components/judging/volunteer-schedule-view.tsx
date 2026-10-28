"use client";

import React, { useState, useMemo } from "react";
import {
  DoorOpen,
  RefreshCw,
  Search,
  CheckCircle2,
  Clock,
  Play,
  ExternalLink,
  Shield,
  Volume2,
} from "lucide-react";
import { useVolunteerSchedule } from "@/hooks/use-judging";
import { VolunteerPanelSchedule } from "@/types/judging";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { SkeletonCard } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";

export function VolunteerScheduleView() {
  const { data: schedule = [], isLoading, isFetching, refetch } = useVolunteerSchedule();
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
    <div className="font-mono space-y-6">
      {/* Volunteer Coordination Header */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 border-b-2 border-black pb-4">
        <div>
          <div className="flex items-center gap-2">
            <DoorOpen className="h-6 w-6 text-black" />
            <h2 className="text-xl font-bold tracking-tight uppercase">
              Volunteer Room & Staging Coordinator
            </h2>
          </div>
          <p className="text-sm text-neutral-600 mt-1">
            Real-time staging schedules, on-deck buffer queues, and room escort monitors for volunteers.
          </p>
        </div>

        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => refetch()}
            disabled={isFetching}
            className="flex items-center gap-1.5 border-black text-black hover:bg-neutral-100"
          >
            <RefreshCw className={`h-4 w-4 ${isFetching ? "animate-spin" : ""}`} />
            REFRESH SCHEDULE
          </Button>
        </div>
      </div>

      {/* Metrics Bar */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <div className="border border-black p-3 bg-white">
          <div className="text-[10px] uppercase text-neutral-500 font-semibold">Active Rooms</div>
          <div className="text-xl font-bold text-black mt-1">
            {stats.activeRooms} / {stats.totalRooms}
          </div>
        </div>
        <div className="border border-black p-3 bg-white">
          <div className="text-[10px] uppercase text-neutral-500 font-semibold flex items-center gap-1">
            <Volume2 className="h-3.5 w-3.5 text-black" />
            Teams On-Deck
          </div>
          <div className="text-xl font-bold text-black mt-1">{stats.onDeckCount}</div>
        </div>
        <div className="border border-black p-3 bg-white">
          <div className="text-[10px] uppercase text-neutral-500 font-semibold flex items-center gap-1">
            <CheckCircle2 className="h-3.5 w-3.5 text-black" />
            Evaluations Done
          </div>
          <div className="text-xl font-bold text-black mt-1">
            {stats.totalCompleted} / {stats.totalTeams}
          </div>
        </div>
        <div className="border border-black p-3 bg-white">
          <div className="text-[10px] uppercase text-neutral-500 font-semibold">Overall Progress</div>
          <div className="text-xl font-bold text-black mt-1">
            {stats.totalTeams > 0
              ? `${Math.round((stats.totalCompleted / stats.totalTeams) * 100)}%`
              : "0%"}
          </div>
        </div>
      </div>

      {/* Toolbar Search & Filter */}
      <div className="border border-black p-3 bg-white flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-neutral-400" />
          <Input
            placeholder="Search teams, rooms, or panels..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="pl-9 border-black font-mono text-sm"
          />
        </div>

        {roomsList.length > 0 && (
          <div className="flex items-center gap-1.5 overflow-x-auto pb-1 sm:pb-0">
            <Button
              variant={selectedRoom === "all" ? "primary" : "outline"}
              size="sm"
              onClick={() => setSelectedRoom("all")}
              className="text-xs uppercase whitespace-nowrap"
            >
              All Rooms
            </Button>
            {roomsList.map((room) => (
              <Button
                key={room}
                variant={selectedRoom === room ? "primary" : "outline"}
                size="sm"
                onClick={() => setSelectedRoom(room)}
                className="text-xs whitespace-nowrap"
              >
                {room}
              </Button>
            ))}
          </div>
        )}
      </div>

      {/* Content Area */}
      {isLoading ? (
        <div className="space-y-4">
          <SkeletonCard />
          <SkeletonCard />
        </div>
      ) : filteredSchedule.length === 0 ? (
        <EmptyState
          title="NO ACTIVE JUDGING SCHEDULES"
          description="There are currently no active judging panels or rooms scheduled. Once an administrator activates a judging round, room staging queues will populate automatically."
        />
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
          {filteredSchedule.map((panel) => (
            <RoomScheduleCard key={panel.panel_id} panel={panel} formatTime={formatTime} />
          ))}
        </div>
      )}
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
    <div className="border-2 border-black bg-white shadow-[3px_3px_0px_0px_rgba(0,0,0,1)] flex flex-col justify-between">
      {/* Card Header */}
      <div className="border-b-2 border-black p-4 bg-neutral-50">
        <div className="flex items-start justify-between gap-2">
          <div>
            <div className="flex items-center gap-2">
              <span className="font-bold text-base uppercase">{panel.room || "Room Unassigned"}</span>
              <Badge variant="outline" className="border-black font-mono text-[10px] uppercase">
                {panel.panel_name}
              </Badge>
            </div>
            <div className="text-xs text-neutral-500 mt-0.5 truncate">
              Round: <strong>{panel.round_name}</strong>
            </div>
          </div>

          <div className="text-right">
            <span className="text-xs font-bold font-mono">
              {panel.completed_count} / {panel.total_count} Evaluated
            </span>
            <div className="w-24 bg-neutral-200 h-1.5 border border-black mt-1 ml-auto overflow-hidden">
              <div
                className="bg-black h-full transition-all"
                style={{ width: `${percentComplete}%` }}
              />
            </div>
          </div>
        </div>

        <div className="flex items-center gap-3 mt-3 text-[11px] text-neutral-600 font-mono">
          <span className="flex items-center gap-1">
            <Clock className="h-3 w-3" />
            {panel.slot_duration_mins}m per slot
          </span>
          <span>•</span>
          <span>Buffer: {panel.buffer_size} on-deck</span>
        </div>
      </div>

      {/* Card Body */}
      <div className="p-4 space-y-4 flex-1">
        {/* 1. CURRENTLY PRESENTING TEAM */}
        <div>
          <div className="text-[10px] uppercase font-bold text-neutral-600 mb-1 flex items-center gap-1">
            <Play className="h-3 w-3 fill-black text-black" />
            Currently In Room (Presenting)
          </div>

          {panel.active_team ? (
            <div className="border-2 border-black p-3 bg-white flex items-start justify-between gap-3 shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]">
              <div>
                <div className="flex items-center gap-2">
                  <span className="font-bold text-sm uppercase">
                    {panel.active_team.team_name}
                  </span>
                  <Badge variant="solid" className="bg-black text-white text-[9px] uppercase">
                    Slot #{panel.active_team.sequence_order}
                  </Badge>
                </div>
                <div className="text-xs text-neutral-500 mt-1 flex items-center gap-1">
                  <Clock className="h-3 w-3" />
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
                        className="p-1 border border-black text-xs hover:bg-neutral-100 inline-flex items-center gap-1"
                      >
                        <span className="capitalize text-[10px] font-bold">{k}</span>
                        <ExternalLink className="h-2.5 w-2.5" />
                      </a>
                    ))}
                  </div>
                )}
            </div>
          ) : (
            <div className="border border-dashed border-neutral-300 p-3 text-center text-xs text-neutral-400">
              No team currently presenting in this room.
            </div>
          )}
        </div>

        {/* 2. ON-DECK QUEUED TEAMS (BUFFER TO ESCORT) */}
        <div>
          <div className="text-[10px] uppercase font-bold text-black mb-1 flex items-center justify-between">
            <span className="flex items-center gap-1">
              <Volume2 className="h-3.5 w-3.5 text-black" />
              On-Deck Staging Buffer ({panel.queued_teams.length} Call-Ups)
            </span>
            <span className="text-[9px] text-neutral-500">Escort to {panel.room}</span>
          </div>

          {panel.queued_teams.length === 0 ? (
            <div className="text-xs text-neutral-400 italic p-2 border border-neutral-200 bg-neutral-50">
              Queue buffer is clear.
            </div>
          ) : (
            <div className="space-y-1.5">
              {panel.queued_teams.map((t, idx) => (
                <div
                  key={t.slot_id}
                  className="border border-black p-2 bg-neutral-50 flex items-center justify-between text-xs"
                >
                  <div className="flex items-center gap-2">
                    <span className="w-5 h-5 bg-black text-white text-[10px] font-bold flex items-center justify-center shrink-0">
                      {idx + 1}
                    </span>
                    <div>
                      <span className="font-bold uppercase">{t.team_name}</span>
                      <span className="text-neutral-500 text-[10px] ml-1.5 font-mono">
                        (Slot #{t.sequence_order})
                      </span>
                    </div>
                  </div>

                  <div className="text-right text-[11px] font-mono text-neutral-600">
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
            <div className="text-[10px] uppercase font-bold text-neutral-500 mb-1">
              Later in Schedule ({panel.upcoming_teams.length} Teams)
            </div>

            <div className="border border-neutral-200 divide-y divide-neutral-200 max-h-32 overflow-y-auto">
              {panel.upcoming_teams.map((t) => (
                <div
                  key={t.slot_id}
                  className="p-1.5 px-2.5 flex items-center justify-between text-xs bg-white text-neutral-700"
                >
                  <div className="flex items-center gap-2 truncate">
                    <span className="text-neutral-400 text-[10px] font-mono">
                      #{t.sequence_order}
                    </span>
                    <span className="truncate">{t.team_name}</span>
                  </div>
                  <span className="text-[10px] text-neutral-400 font-mono shrink-0">
                    {formatTime(t.estimated_start_time)}
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Card Footer */}
      <div className="border-t border-black p-2.5 bg-neutral-50 text-[11px] text-neutral-500 flex items-center justify-between font-mono">
        <span className="flex items-center gap-1">
          <Shield className="h-3 w-3" />
          Live queue updates every 10s
        </span>
        <span className="uppercase font-bold text-black">
          Panel ID: {panel.panel_id.substring(0, 8)}...
        </span>
      </div>
    </div>
  );
}
