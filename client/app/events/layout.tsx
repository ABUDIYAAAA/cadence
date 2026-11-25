import React from "react";
import EventsLayoutClient from "./events-layout-client";

export const instant = false;

export default function EventsLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return <EventsLayoutClient>{children}</EventsLayoutClient>;
}
