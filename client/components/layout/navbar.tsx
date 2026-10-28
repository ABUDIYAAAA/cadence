"use client";

import React from "react";
import Link from "next/link";
import { useAuth } from "@/providers/auth-provider";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

export function Navbar() {
  const { user, isAuthenticated, logout, isAdmin, isJudge, isVolunteer } = useAuth();

  return (
    <header className="border-b border-black bg-white font-mono sticky top-0 z-40">
      <div className="mx-auto flex h-14 max-w-7xl items-center justify-between px-4 sm:px-6">
        {/* Brand */}
        <div className="flex items-center gap-6">
          <Link href="/" className="flex items-center gap-2 font-bold tracking-widest text-lg uppercase">
            <span className="bg-black text-white px-1.5 py-0.5 text-xs">CD</span>
            <span>CADENCE</span>
          </Link>

          {/* Navigation Links */}
          <nav className="hidden md:flex items-center gap-4 text-xs uppercase tracking-wider">
            <Link
              href="/"
              className="text-neutral-600 hover:text-black transition-colors"
            >
              Overview
            </Link>
            {isAuthenticated && (
              <>
                <Link
                  href="/tickets"
                  className="text-neutral-600 hover:text-black transition-colors"
                >
                  Tickets
                </Link>
                <Link
                  href="/teams"
                  className="text-neutral-600 hover:text-black transition-colors"
                >
                  Teams
                </Link>
                <Link
                  href="/judging"
                  className="text-neutral-600 hover:text-black transition-colors"
                >
                  Judging
                </Link>
                {(isAdmin || isVolunteer || isJudge) && (
                  <Link
                    href="/users"
                    className="text-neutral-600 hover:text-black transition-colors"
                  >
                    Users
                  </Link>
                )}
                {isAdmin && (
                  <Link
                    href="/audit"
                    className="text-neutral-600 hover:text-black transition-colors"
                  >
                    Audit
                  </Link>
                )}
              </>
            )}
          </nav>
        </div>

        {/* User state */}
        <div className="flex items-center gap-3">
          {isAuthenticated && user ? (
            <div className="flex items-center gap-3">
              <div className="hidden sm:flex flex-col text-right">
                <span className="text-xs font-semibold text-black leading-tight">
                  {user.name}
                </span>
                <span className="text-[10px] text-neutral-500 leading-tight">
                  {user.email}
                </span>
              </div>
              <Badge variant="solid" size="sm">
                {user.role}
              </Badge>
              <Button
                variant="outline"
                size="sm"
                onClick={() => logout()}
                className="text-xs h-7 px-2.5"
              >
                Logout
              </Button>
            </div>
          ) : (
            <div className="flex items-center gap-2">
              <Link href="/login">
                <Button variant="outline" size="sm" className="text-xs h-7">
                  Log in
                </Button>
              </Link>
            </div>
          )}
        </div>
      </div>
    </header>
  );
}
