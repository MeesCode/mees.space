"use client";

import { NavigationProvider } from "@/lib/navigation";
import { ReactNode } from "react";

export function Providers({ children }: { children: ReactNode }) {
  return <NavigationProvider>{children}</NavigationProvider>;
}
