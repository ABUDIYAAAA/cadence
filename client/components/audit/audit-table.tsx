"use client";

import React from "react";
import { Eye, Globe, Smartphone } from "lucide-react";
import { AuditLogResponse } from "@/types/audit";
import { getAuditActionBadgeVariant } from "@/lib/audit-registry";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";

interface AuditTableProps {
  logs: AuditLogResponse[];
  isLoading: boolean;
  onView: (log: AuditLogResponse) => void;
}

export function AuditTable({ logs, isLoading, onView }: AuditTableProps) {
  if (isLoading) {
    return (
      <div className="w-full overflow-x-auto font-sans">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Timestamp</TableHead>
              <TableHead>Action</TableHead>
              <TableHead>Actor</TableHead>
              <TableHead>Target</TableHead>
              <TableHead>Description</TableHead>
              <TableHead>Client / IP</TableHead>
              <TableHead className="text-right">Inspect</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {Array.from({ length: 6 }).map((_, i) => (
              <TableRow key={i}>
                <TableCell>
                  <div className="space-y-1.5">
                    <Skeleton className="h-3.5 w-28" />
                    <Skeleton className="h-2.5 w-16" />
                  </div>
                </TableCell>
                <TableCell>
                  <Skeleton className="h-5 w-24 rounded-full" />
                </TableCell>
                <TableCell>
                  <Skeleton className="h-3.5 w-32" />
                </TableCell>
                <TableCell>
                  <Skeleton className="h-3.5 w-20" />
                </TableCell>
                <TableCell>
                  <Skeleton className="h-3.5 w-48" />
                </TableCell>
                <TableCell>
                  <Skeleton className="h-3.5 w-24" />
                </TableCell>
                <TableCell className="text-right">
                  <Skeleton className="h-7 w-7 rounded-[6px] ml-auto" />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    );
  }

  if (logs.length === 0) {
    return (
      <EmptyState
        title="No audit records found"
        description="No audit logs matched your current filters. Adjust filters or date boundaries to expand results."
      />
    );
  }

  return (
    <div className="w-full overflow-x-auto font-sans">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Timestamp</TableHead>
            <TableHead>Action</TableHead>
            <TableHead>Actor</TableHead>
            <TableHead>Target</TableHead>
            <TableHead>Description</TableHead>
            <TableHead>Client / IP</TableHead>
            <TableHead className="text-right">Inspect</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {logs.map((log) => {
            const date = new Date(log.created_at);

            return (
              <TableRow key={log.id} className="group">
                {/* Timestamp */}
                <TableCell className="font-mono text-xs whitespace-nowrap">
                  <div className="font-semibold text-[var(--ink-primary)]">
                    {date.toLocaleDateString()}{" "}
                    <span className="text-[var(--ink-secondary)]">
                      {date.toLocaleTimeString()}
                    </span>
                  </div>
                  <span className="text-[10px] text-[var(--ink-tertiary)]">
                    {log.id.slice(0, 8)}...
                  </span>
                </TableCell>

                {/* Action */}
                <TableCell className="whitespace-nowrap">
                  <Badge variant={getAuditActionBadgeVariant(log.action)} size="sm">
                    {log.action}
                  </Badge>
                </TableCell>

                {/* Actor */}
                <TableCell className="text-xs">
                  {log.actor_email ? (
                    <div className="flex flex-col">
                      <span className="font-bold text-[var(--ink-primary)] truncate max-w-[150px]">
                        {log.actor_email}
                      </span>
                      {log.actor_role && (
                        <span className="text-[10px] uppercase text-[var(--ink-secondary)]">
                          {log.actor_role}
                        </span>
                      )}
                    </div>
                  ) : (
                    <span className="text-[var(--ink-tertiary)] italic text-[11px]">
                      System / Unauthenticated
                    </span>
                  )}
                </TableCell>

                {/* Target */}
                <TableCell className="text-xs whitespace-nowrap">
                  <div className="flex items-center gap-1.5">
                    <span className="text-[10px] bg-[var(--card-solid-subtle)] border border-[var(--card-line)] text-[var(--ink-secondary)] px-1.5 py-0.5 rounded-[6px] uppercase font-medium">
                      {log.target_type}
                    </span>
                    {log.target_id && (
                      <span className="text-[10px] text-[var(--ink-tertiary)] font-mono">
                        #{log.target_id.slice(0, 8)}
                      </span>
                    )}
                  </div>
                </TableCell>

                {/* Description */}
                <TableCell className="text-xs">
                  <span className="line-clamp-2 text-[var(--ink-primary)] font-sans">
                    {log.description}
                  </span>
                </TableCell>

                {/* IP & Device */}
                <TableCell className="text-xs whitespace-nowrap text-[var(--ink-secondary)]">
                  <div className="space-y-0.5">
                    {log.ip_address && (
                      <div className="flex items-center gap-1 font-mono text-[11px] text-[var(--ink-primary)]">
                        <Globe className="h-3 w-3 text-[var(--color-blue)]" />
                        <span>{log.ip_address}</span>
                      </div>
                    )}
                    {log.device_id && (
                      <div className="flex items-center gap-1 text-[10px] text-[var(--ink-tertiary)]">
                        <Smartphone className="h-2.5 w-2.5" />
                        <span>{log.device_id.slice(0, 10)}...</span>
                      </div>
                    )}
                    {!log.ip_address && !log.device_id && (
                      <span className="text-[var(--ink-tertiary)] text-[10px]">—</span>
                    )}
                  </div>
                </TableCell>

                {/* Actions */}
                <TableCell className="text-right">
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => onView(log)}
                    title="Inspect Log Payload & Metadata"
                    className="h-7 w-7 p-0 text-[var(--ink-secondary)] hover:text-[var(--ink-primary)]"
                  >
                    <Eye className="h-3.5 w-3.5" />
                  </Button>
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </div>
  );
}
