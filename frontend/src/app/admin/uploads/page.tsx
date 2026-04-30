"use client";

import { useEffect, useState } from "react";
import { apiFetch } from "@/lib/api";
import { logout } from "@/lib/auth";
import { ImageInfo } from "@/lib/types";

export default function UploadsPage() {
  const [images, setImages] = useState<ImageInfo[]>([]);
  const [selected, setSelected] = useState<string | null>(null);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const res = await apiFetch("/api/images");
      if (!res.ok) {
        setLoaded(true);
        return;
      }
      const data: ImageInfo[] = await res.json();
      if (!cancelled) {
        setImages(data);
        setLoaded(true);
      }
    })();
    return () => { cancelled = true; };
  }, []);

  const totalBytes = images.reduce((n, im) => n + im.size, 0);
  const unused = images.filter((im) => im.ref_count === 0);
  const unusedBytes = unused.reduce((n, im) => n + im.size, 0);

  return (
    <div style={{ display: "flex", height: "100vh", overflow: "hidden", color: "var(--color)" }}>
      {/* Left rail */}
      <div style={leftRailStyle}>
        <div style={headerStyle}>
          <span style={{ color: "var(--accent)", fontWeight: "bold", fontSize: "0.9rem" }}>
            uploads
          </span>
          <span style={{ display: "flex", gap: 10 }}>
            <a href="/" style={navLinkStyle}>site</a>
            <a href="/admin/editor" style={navLinkStyle}>editor</a>
            <button onClick={logout} style={{ ...navLinkStyle, background: "none", border: "none", cursor: "pointer" }}>
              logout
            </button>
          </span>
        </div>
      </div>

      {/* Grid + totals */}
      <div style={{ flex: 1, padding: 14, overflow: "auto" }}>
        <div data-testid="totals" style={totalsStyle}>
          {loaded
            ? `${images.length} images · ${formatBytes(totalBytes)} total · ${unused.length} unused (${formatBytes(unusedBytes)})`
            : "loading…"}
        </div>
        <div style={gridStyle}>
          {images.map((im) => (
            <div
              key={im.filename}
              onClick={() => setSelected(im.filename)}
              style={{
                position: "relative",
                cursor: "pointer",
                outline: selected === im.filename ? "2px solid var(--accent)" : "none",
              }}
            >
              <img
                src={im.url}
                alt={im.filename}
                loading="lazy"
                style={{
                  width: "100%",
                  aspectRatio: "1",
                  objectFit: "cover",
                  background: "#1a1a1a",
                  display: "block",
                }}
              />
              <div
                data-testid={`ref-badge-${im.filename}`}
                title={refTitle(im.ref_count)}
                style={badgeStyle(im.ref_count)}
              >
                {refLabel(im.ref_count)}
              </div>
              <div style={filenameStyle}>{im.filename}</div>
            </div>
          ))}
        </div>
      </div>

      {/* Detail rail (skeleton — populated by later tasks) */}
      <div style={detailRailStyle}>
        {selected ? <span style={{ fontSize: 12 }}>{selected}</span> : null}
      </div>
    </div>
  );
}

function refLabel(count: number): string {
  if (count < 0) return "?";
  return String(count);
}

function refTitle(count: number): string {
  if (count < 0) return "Couldn't compute references";
  if (count === 0) return "Not referenced anywhere";
  return `Used in ${count} page${count === 1 ? "" : "s"}`;
}

function badgeStyle(count: number): React.CSSProperties {
  let bg = "var(--accent)";
  if (count === 0) bg = "#ff6b6b";
  if (count < 0) bg = "rgba(255,255,255,0.3)";
  return {
    position: "absolute",
    top: 4,
    right: 4,
    background: bg,
    color: "#000",
    fontSize: 10,
    padding: "1px 5px",
    borderRadius: 2,
  };
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}

const leftRailStyle: React.CSSProperties = {
  width: 170,
  borderRight: "1px solid rgba(255,255,255,0.08)",
  padding: 14,
  flexShrink: 0,
  overflow: "auto",
};

const headerStyle: React.CSSProperties = {
  display: "flex",
  justifyContent: "space-between",
  alignItems: "center",
  marginBottom: 16,
};

const navLinkStyle: React.CSSProperties = {
  color: "rgba(255,255,255,0.4)",
  textDecoration: "none",
  fontFamily: "inherit",
  fontSize: "0.75rem",
};

const totalsStyle: React.CSSProperties = {
  fontSize: 11,
  color: "rgba(255,255,255,0.5)",
  marginBottom: 10,
};

const gridStyle: React.CSSProperties = {
  display: "grid",
  gridTemplateColumns: "repeat(auto-fill, minmax(120px, 1fr))",
  gap: 8,
};

const filenameStyle: React.CSSProperties = {
  fontSize: 10,
  color: "rgba(255,255,255,0.45)",
  marginTop: 3,
  whiteSpace: "nowrap",
  overflow: "hidden",
  textOverflow: "ellipsis",
};

const detailRailStyle: React.CSSProperties = {
  width: 230,
  borderLeft: "1px solid rgba(255,255,255,0.08)",
  padding: 14,
  flexShrink: 0,
};
