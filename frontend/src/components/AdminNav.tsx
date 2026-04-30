"use client";

import { logout } from "@/lib/auth";

type Section = "editor" | "uploads" | "settings";

const items: { key: Section | "site"; label: string; href: string }[] = [
  { key: "site", label: "site", href: "/" },
  { key: "editor", label: "editor", href: "/admin/editor" },
  { key: "uploads", label: "uploads", href: "/admin/uploads" },
  { key: "settings", label: "settings", href: "/admin/settings" },
];

export function AdminNav({ current }: { current: Section }) {
  return (
    <nav style={{ marginBottom: 18 }}>
      {items.map((item) => {
        const active = item.key === current;
        return (
          <a
            key={item.key}
            href={item.href}
            style={{
              display: "block",
              padding: "4px 8px",
              borderRadius: 3,
              marginBottom: 2,
              fontSize: "0.85rem",
              color: active ? "var(--accent)" : "rgba(255,255,255,0.6)",
              backgroundColor: active ? "rgba(51,172,183,0.1)" : "transparent",
              fontWeight: active ? "bold" : "normal",
              textDecoration: "none",
              fontFamily: "inherit",
            }}
          >
            {item.label}
          </a>
        );
      })}
      <button
        onClick={logout}
        style={{
          display: "block",
          width: "100%",
          textAlign: "left",
          padding: "4px 8px",
          borderRadius: 3,
          marginTop: 2,
          fontSize: "0.85rem",
          color: "rgba(255,255,255,0.6)",
          background: "none",
          border: "none",
          fontFamily: "inherit",
          cursor: "pointer",
        }}
      >
        logout
      </button>
    </nav>
  );
}
