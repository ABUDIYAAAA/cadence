"use client";

import React from "react";
import { Eye, Globe, Smartphone } from "lucide-react";
import { AuditLogResponse } from "@/types/audit";
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
      <div className="border border-neutral-300 bg-white p-4 font-mono space-y-3">
        <div className="space-y-2">
          {Array.from({ length: 8 }).map((_, i) => (
            <div key={i} className="flex items-center space-x-4 py-2">
              <Skeleton className="h-4 w-1/6" />
              <Skeleton className="h-4 w-1/6" />
              <Skeleton className="h-4 w-1/4" />
              <Skeleton className="h-4 w-1/4" />
              <Skeleton className="h-4 w-1/6" />
            </div>
          ))}
        </div>
      </div>
    );
  }

  if (logs.length === 0) {
    return (
      <EmptyState
        title="No Audit Records Found"
        description="No audit logs matched your query parameters. Adjust filters or date boundaries to expand results."
      />
    );
  }

  const getActionBadgeVariant = (action: string) => {
    if (action.includes("delete") || action.includes("failure") || action.includes("duplicate")) {
      return "solid";
    }
    if (action.includes("create") || action.includes("success") || action.includes("activate")) {
      return "outline";
    }
    return "subtle";
  };

  return (
    <div className="border border-neutral-300 bg-white overflow-hidden font-mono">
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
              <TableRow key={log.id} className="group hover:bg-neutral-50/80">
                {/* Timestamp */}
                <TableCell className="font-mono text-xs whitespace-nowrap">
                  <div className="font-semibold text-black">
                    {date.toLocaleDateString()}{" "}
                    <span className="text-neutral-500">
                      {date.toLocaleTimeString()}
                    </span>
                  </div>
                  <span className="text-[10px] text-neutral-400">
                    {log.id.slice(0, 8)}...
                  </span>
                </TableCell>

                {/* Action */}
                <TableCell className="whitespace-nowrap">
                  <Badge variant={getActionBadgeVariant(log.action)} size="sm">
                    {log.action}
                  </Badge>
                </TableCell>

                {/* Actor */}
                <TableCell className="text-xs">
                  {log.actor_email ? (
                    <div className="flex flex-col">
                      <span className="font-bold text-black truncate max-w-[150px]">
                        {log.actor_email}
                      </span>
                      {log.actor_role && (
                        <span className="text-[10px] uppercase text-neutral-500">
                          {log.actor_role}
                        </span>
                      )}
                    </div>
                  ) : (
                    <span className="text-neutral-400 italic text-[11px]">
                      System / Unauthenticated
                    </span>
                  )}
                </TableCell>

                {/* Target */}
                <TableCell className="text-xs whitespace-nowrap">
                  <div className="flex items-center gap-1.5">
                    <span className="text-[10px] bg-neutral-100 border border-neutral-300 px-1.5 py-0.2 uppercase font-medium">
                      {log.target_type}
                    </span>
                    {log.target_id && (
                      <span className="text-[10px] text-neutral-500 font-mono">
                        #{log.target_id.slice(0, 8)}
                      </span>
                    )}
                  </div>
                </TableCell>

                {/* Description */}
                <TableCell className="text-xs">
                  <span className="line-clamp-2 text-neutral-800 font-sans sm:font-mono">
                    {log.description}
                  </span>
                </TableCell>

                {/* IP & Device */}
                <TableCell className="text-xs whitespace-nowrap text-neutral-500">
                  <div className="space-y-0.5">
                    {log.ip_address && (
                      <div className="flex items-center gap-1 font-mono text-[11px] text-black">
                        <Globe className="h-3 w-3 text-neutral-400" />
                        <span>{log.ip_address}</span>
                      </div>
                    )}
                    {log.device_id && (
                      <div className="flex items-center gap-1 text-[10px] text-neutral-400">
                        <Smartphone className="h-2.5 w-2.5" />
                        <span>{log.device_id.slice(0, 10)}...</span>
                      </div>
                    )}
                    {!log.ip_address && !log.device_id && (
                      <span className="text-neutral-300 text-[10px]">—</span>
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
                    className="h-7 w-7 p-0"
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
