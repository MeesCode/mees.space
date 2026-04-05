"use client";

import { useEffect, useState } from "react";
import { usePathname } from "next/navigation";
import { isLoggedIn } from "@/lib/auth";

export default function AdminLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const pathname = usePathname();
  const [checked, setChecked] = useState(false);

  useEffect(() => {
    if (pathname === "/admin/login") {
      setChecked(true);
      return;
    }
    if (!isLoggedIn()) {
      window.location.replace("/admin/login");
      return;
    } else {
      setChecked(true);
    }
  }, [pathname]);

  if (!checked && pathname !== "/admin/login") {
    return null;
  }

  return (
    <div
      style={{
        position: "fixed",
        inset: 0,
        zIndex: 1000,
        backgroundColor: "var(--app-background)",
        overflow: "auto",
      }}
    >
      {children}
    </div>
  );
}
