"use client";

import React, { useState } from "react";
import Link from "next/link";
import { useAuth } from "@/providers/auth-provider";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Alert } from "@/components/ui/alert";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Input } from "@/components/ui/input";

export default function HomePage() {
  const { user, isAuthenticated, isLoading } = useAuth();
  const [testInput, setTestInput] = useState("");

  const apiEndpoints = [
    { module: "Auth", path: "/api/v1/auth/login", method: "POST", auth: "Public" },
    { module: "Users", path: "/api/v1/users", method: "GET", auth: "Admin/Judge/Volunteer" },
    { module: "Teams", path: "/api/v1/teams/my-team", method: "GET", auth: "Authenticated" },
    { module: "Tickets", path: "/api/v1/tickets/analytics", method: "GET", auth: "Staff" },
    { module: "Judging", path: "/api/v1/judging/judge/panels", method: "GET", auth: "Judge/Admin" },
    { module: "Notifications", path: "/api/v1/notifications/inbox", method: "GET", auth: "Authenticated" },
    { module: "Audit", path: "/api/v1/audit-logs", method: "GET", auth: "Admin" },
  ];

  return (
    <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 space-y-8 font-mono">
      {/* Top Banner */}
      <div className="border border-black p-6 bg-white flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-2 mb-2">
            <span className="text-xs bg-black text-white px-2 py-0.5">SYSTEM ACTIVE</span>
            <span className="text-xs text-neutral-500 font-mono">ENV: {process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1"}</span>
          </div>
          <h1 className="text-2xl font-bold tracking-tight uppercase">Cadence Control Panel</h1>
          <p className="text-xs text-neutral-600 mt-1">
            Monochrome frontend interface connected to Cadence v1 high-performance API.
          </p>
        </div>

        <div className="flex items-center gap-2">
          {isAuthenticated && user ? (
            <div className="text-right">
              <p className="text-xs font-semibold uppercase">{user.name}</p>
              <Badge variant="solid" size="sm" className="mt-1">
                {user.role}
              </Badge>
            </div>
          ) : (
            <Link href="/login">
              <Button variant="primary" size="md">
                Sign In
              </Button>
            </Link>
          )}
        </div>
      </div>

      {/* Notice */}
      <Alert variant="info" title="CLIENT STACK INITIALIZED">
        TanStack React Query, Axios with credentials, typed API interfaces matching all backend DTOs,
        and monochrome component architecture are loaded.
      </Alert>

      {/* Tabs Section */}
      <Tabs defaultValue="architecture">
        <TabsList>
          <TabsTrigger value="architecture">Architecture & API Spec</TabsTrigger>
          <TabsTrigger value="components">UI Component Kit</TabsTrigger>
          <TabsTrigger value="status">Session Status</TabsTrigger>
        </TabsList>

        {/* Tab 1: Architecture */}
        <TabsContent value="architecture">
          <Card>
            <CardHeader>
              <CardTitle>Connected API Modules</CardTitle>
              <CardDescription>
                Endpoints configured with automatic credentials, CSRF, and typed schema interfaces.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Module</TableHead>
                    <TableHead>Method</TableHead>
                    <TableHead>Endpoint</TableHead>
                    <TableHead>Access Level</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {apiEndpoints.map((item) => (
                    <TableRow key={item.path}>
                      <TableCell className="font-bold">{item.module}</TableCell>
                      <TableCell>
                        <Badge variant="solid" size="sm">
                          {item.method}
                        </Badge>
                      </TableCell>
                      <TableCell className="font-mono text-xs">{item.path}</TableCell>
                      <TableCell>
                        <Badge variant="subtle" size="sm">
                          {item.auth}
                        </Badge>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>

        {/* Tab 2: Components Showcase */}
        <TabsContent value="components">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <Card>
              <CardHeader>
                <CardTitle>Button System</CardTitle>
                <CardDescription>Strict monochrome variants</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex flex-wrap gap-2">
                  <Button variant="primary">Primary</Button>
                  <Button variant="secondary">Secondary</Button>
                  <Button variant="outline">Outline</Button>
                  <Button variant="ghost">Ghost</Button>
                  <Button variant="destructive">Destructive</Button>
                  <Button variant="primary" isLoading>Loading</Button>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Input & Form Controls</CardTitle>
                <CardDescription>Sharp borders and mono typography</CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                <Input
                  label="Search or Action"
                  placeholder="Type command..."
                  value={testInput}
                  onChange={(e) => setTestInput(e.target.value)}
                  helperText="Interactive input element"
                />
                <Input
                  label="Field with Validation"
                  placeholder="Required field..."
                  defaultValue="invalid_entry"
                  error="Format must comply with API requirements"
                />
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Badges & Status</CardTitle>
                <CardDescription>Clean monochrome indicators</CardDescription>
              </CardHeader>
              <CardContent className="flex flex-wrap gap-2">
                <Badge variant="solid">Solid Black</Badge>
                <Badge variant="outline">Outline</Badge>
                <Badge variant="subtle">Subtle Gray</Badge>
                <Badge variant="solid" size="sm">Small Solid</Badge>
                <Badge variant="outline" size="sm">Small Outline</Badge>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Callouts & Alerts</CardTitle>
                <CardDescription>System messaging</CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                <Alert variant="warning" title="ATTENTION">
                  Scores will automatically lock after slot completion.
                </Alert>
                <Alert variant="error" title="ERROR RECORD">
                  Duplicate ticket check-in prevented by gatekeeper.
                </Alert>
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        {/* Tab 3: Session Status */}
        <TabsContent value="status">
          <Card>
            <CardHeader>
              <CardTitle>Authentication & Device State</CardTitle>
              <CardDescription>Real-time session metadata via HTTP cookies</CardDescription>
            </CardHeader>
            <CardContent>
              {isLoading ? (
                <p className="text-xs text-neutral-500">Checking authentication status...</p>
              ) : isAuthenticated && user ? (
                <div className="space-y-2 text-xs">
                  <div className="flex justify-between py-1 border-b border-neutral-200">
                    <span className="text-neutral-600">User ID</span>
                    <span className="font-mono">{user.id}</span>
                  </div>
                  <div className="flex justify-between py-1 border-b border-neutral-200">
                    <span className="text-neutral-600">Email</span>
                    <span className="font-mono">{user.email}</span>
                  </div>
                  <div className="flex justify-between py-1 border-b border-neutral-200">
                    <span className="text-neutral-600">Role</span>
                    <Badge variant="solid" size="sm">{user.role}</Badge>
                  </div>
                  <div className="flex justify-between py-1 border-b border-neutral-200">
                    <span className="text-neutral-600">Password Configured</span>
                    <span>{user.is_password_configured ? "Yes" : "No (Setup Needed)"}</span>
                  </div>
                </div>
              ) : (
                <div className="space-y-3">
                  <p className="text-xs text-neutral-600">
                    Currently unauthenticated. Cookies will be attached automatically when logging in.
                  </p>
                  <Link href="/login">
                    <Button variant="primary" size="sm">
                      Go to Login
                    </Button>
                  </Link>
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
