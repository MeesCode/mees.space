"use client";

import { logout } from "@/lib/auth";

type Section = "editor" | "uploads" | "settings";

const primary: { key: Section; label: string; href: string }[] = [
  { key: "editor", label: "editor", href: "/admin/editor" },
  { key: "uploads", label: "uploads", href: "/admin/uploads" },
  { key: "settings", label: "settings", href: "/admin/settings" },
];

const linkStyle = (active: boolean): React.CSSProperties => ({
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
});

export function AdminNav({ current }: { current: Section }) {
  return (
    <nav>
      {primary.map((item) => (
        <a key={item.key} href={item.href} style={linkStyle(item.key === current)}>
          {item.label}
        </a>
      ))}
    </nav>
  );
}

export function AdminNavFooter() {
  return (
    <nav style={{ paddingTop: 10, borderTop: "1px solid rgba(255,255,255,0.08)" }}>
      <a href="/" style={linkStyle(false)}>
        site
      </a>
      <button
        onClick={logout}
        style={{
          ...linkStyle(false),
          width: "100%",
          textAlign: "left",
          background: "none",
          border: "none",
          cursor: "pointer",
        }}
      >
        logout
      </button>
    </nav>
  );
}
