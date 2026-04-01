"use client";

import { usePathname } from "next/navigation";
import { Sidebar } from "./Sidebar";
import { Minimap } from "./Minimap";
import { ReactNode } from "react";

export function ClientLayout({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const isAdmin = pathname?.startsWith("/admin");

  if (isAdmin) {
    return <>{children}</>;
  }

  return (
    <>
      <Sidebar />
      <main id="article-wrapper" className="app-container">
        {children}
      </main>
      <Minimap />
    </>
  );
}
