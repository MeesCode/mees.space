"use client";

import { useEffect, useMemo, useState } from "react";
import { apiFetch } from "@/lib/api";
import { logout } from "@/lib/auth";
import { ImageInfo, ImageRefs } from "@/lib/types";

type View = "all" | "unused";
type Sort = "newest" | "name" | "size";

export default function UploadsPage() {
  const [images, setImages] = useState<ImageInfo[]>([]);
  const [view, setView] = useState<View>("all");
  const [sort, setSort] = useState<Sort>("newest");
  const [selected, setSelected] = useState<string | null>(null);
  const [refs, setRefs] = useState<string[] | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [pendingDelete, setPendingDelete] = useState<{ filename: string; pages: string[] } | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const res = await apiFetch("/api/images");
      if (!res.ok) { setLoaded(true); return; }
      const data: ImageInfo[] = await res.json();
      if (!cancelled) { setImages(data); setLoaded(true); }
    })();
    return () => { cancelled = true; };
  }, []);

  // Load refs whenever the selected image changes.
  useEffect(() => {
    if (!selected) { setRefs(null); return; }
    let cancelled = false;
    (async () => {
      const res = await apiFetch(`/api/images/${encodeURIComponent(selected)}/refs`);
      if (!res.ok) { if (!cancelled) setRefs([]); return; }
      const data: ImageRefs = await res.json();
      if (!cancelled) setRefs(data.pages);
    })();
    return () => { cancelled = true; };
  }, [selected]);

  const visible = useMemo(() => {
    let arr = view === "unused" ? images.filter((i) => i.ref_count === 0) : images.slice();
    arr.sort((a, b) => {
      if (sort === "name") return a.filename.localeCompare(b.filename);
      if (sort === "size") return b.size - a.size;
      // newest = uploaded_at desc
      return b.uploaded_at.localeCompare(a.uploaded_at);
    });
    return arr;
  }, [images, view, sort]);

  const totalBytes = images.reduce((n, im) => n + im.size, 0);
  const unusedCount = images.filter((im) => im.ref_count === 0).length;
  const unusedBytes = images
    .filter((im) => im.ref_count === 0)
    .reduce((n, im) => n + im.size, 0);

  const selectedInfo = images.find((im) => im.filename === selected) ?? null;

  const copyUrl = async () => {
    if (!selectedInfo) return;
    await navigator.clipboard?.writeText(selectedInfo.url);
  };

  const requestDelete = async (filename: string) => {
    // Use grid info to short-circuit the obvious "no refs" path.
    const info = images.find((i) => i.filename === filename);
    if (info && info.ref_count === 0) {
      const res = await apiFetch(`/api/images/${encodeURIComponent(filename)}`, { method: "DELETE" });
      if (res.status === 204) {
        setImages((prev) => prev.filter((i) => i.filename !== filename));
        setSelected((s) => (s === filename ? null : s));
        return;
      }
      if (res.status === 409) {
        const body = await res.json().catch(() => ({ pages: [] as string[] }));
        setPendingDelete({ filename, pages: body.pages ?? [] });
        return;
      }
      if (res.status === 404) {
        setImages((prev) => prev.filter((i) => i.filename !== filename));
        setSelected((s) => (s === filename ? null : s));
        return;
      }
      return;
    }

    // Has refs (or unknown). Always confirm.
    const refsRes = await apiFetch(`/api/images/${encodeURIComponent(filename)}/refs`);
    const pages = refsRes.ok ? ((await refsRes.json()) as ImageRefs).pages : [];
    setPendingDelete({ filename, pages });
  };

  const confirmForceDelete = async () => {
    if (!pendingDelete) return;
    const res = await apiFetch(
      `/api/images/${encodeURIComponent(pendingDelete.filename)}?force=1`,
      { method: "DELETE" },
    );
    if (res.status === 204 || res.status === 404) {
      setImages((prev) => prev.filter((i) => i.filename !== pendingDelete.filename));
      setSelected((s) => (s === pendingDelete.filename ? null : s));
    }
    setPendingDelete(null);
  };

  const uploadFile = async (file: File) => {
    const form = new FormData();
    form.append("file", file);
    const res = await apiFetch("/api/images", { method: "POST", body: form });
    if (res.ok) {
      const info: ImageInfo = await res.json();
      setImages((prev) => [info, ...prev.filter((i) => i.filename !== info.filename)]);
    }
  };

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

        <div style={sectionLabel}>view</div>
        <RailRow active={view === "all"} onClick={() => setView("all")} testid="filter-all">
          ★ all images <span style={{ float: "right", color: "rgba(255,255,255,0.4)" }}>{images.length}</span>
        </RailRow>
        <RailRow active={view === "unused"} onClick={() => setView("unused")} testid="filter-unused">
          unused only <span style={{ float: "right", color: "#ff6b6b" }}>{unusedCount}</span>
        </RailRow>

        <div style={sectionLabel}>sort</div>
        <RailRow active={sort === "newest"} onClick={() => setSort("newest")} testid="sort-newest">↓ newest</RailRow>
        <RailRow active={sort === "name"} onClick={() => setSort("name")} testid="sort-name">name</RailRow>
        <RailRow active={sort === "size"} onClick={() => setSort("size")} testid="sort-size">size</RailRow>

        <div
          data-testid="dropzone"
          onDragOver={(e) => { e.preventDefault(); }}
          onDrop={(e) => {
            e.preventDefault();
            const files = Array.from(e.dataTransfer?.files ?? []);
            files.forEach((f) => uploadFile(f));
          }}
          onClick={() => document.getElementById("upload-input")?.click()}
          style={dropzoneStyle}
        >
          ⬆ drag &amp; drop<br />or click to upload
          <input
            id="upload-input"
            type="file"
            accept="image/*"
            multiple
            style={{ display: "none" }}
            onChange={(e) => {
              const files = Array.from(e.target.files ?? []);
              files.forEach((f) => uploadFile(f));
              e.currentTarget.value = "";
            }}
          />
        </div>
      </div>

      {/* Grid + totals */}
      <div style={{ flex: 1, padding: 14, overflow: "auto" }}>
        <div data-testid="totals" style={totalsStyle}>
          {loaded
            ? `${images.length} images · ${formatBytes(totalBytes)} total · ${unusedCount} unused (${formatBytes(unusedBytes)})`
            : "loading…"}
        </div>
        <div style={gridStyle}>
          {visible.map((im) => (
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

      {/* Confirm delete modal */}
      {pendingDelete && (
        <div data-testid="confirm-modal" style={modalBackdrop}>
          <div style={modalBoxStyle}>
            <div style={{ fontSize: 13, marginBottom: 10 }}>
              Delete <code>{pendingDelete.filename}</code>?
            </div>
            <div style={{ fontSize: 12, color: "rgba(255,255,255,0.6)", marginBottom: 8 }}>
              Still referenced by:
            </div>
            <div style={{ marginBottom: 14, maxHeight: 160, overflow: "auto" }}>
              {pendingDelete.pages.map((p) => (
                <a
                  key={p}
                  href={`/admin/editor?path=${encodeURIComponent(p)}`}
                  style={{ display: "block", color: "rgba(255,255,255,0.85)", textDecoration: "none", fontSize: 11, padding: "3px 0" }}
                >
                  /{p}
                </a>
              ))}
            </div>
            <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
              <button onClick={() => setPendingDelete(null)} style={cancelButton}>Cancel</button>
              <button data-testid="confirm-delete" onClick={confirmForceDelete} style={dangerButton}>Delete anyway</button>
            </div>
          </div>
        </div>
      )}

      {/* Detail rail */}
      <div style={detailRailStyle}>
        {selectedInfo ? (
          <>
            <div style={sectionLabel}>selected</div>
            <img
              src={selectedInfo.url}
              alt={selectedInfo.filename}
              style={{ width: "100%", aspectRatio: "1", objectFit: "cover", background: "#1a1a1a", marginBottom: 10 }}
            />
            <div data-testid="detail-filename" style={{ fontSize: 11, wordBreak: "break-all", marginBottom: 4 }}>
              {selectedInfo.filename}
            </div>
            <div data-testid="detail-size" style={{ fontSize: 10, color: "rgba(255,255,255,0.4)", marginBottom: 14 }}>
              {formatBytes(selectedInfo.size)} · {selectedInfo.uploaded_at.slice(0, 10)}
            </div>

            <div style={sectionLabel}>used in ({refs?.length ?? "…"})</div>
            <div style={{ marginBottom: 18 }}>
              {refs?.length === 0 && (
                <div style={{ fontSize: 11, color: "rgba(255,255,255,0.4)" }}>not referenced</div>
              )}
              {refs?.map((page) => (
                <a
                  key={page}
                  href={`/admin/editor?path=${encodeURIComponent(page)}`}
                  style={{ display: "block", color: "rgba(255,255,255,0.7)", textDecoration: "none", fontSize: 11, padding: "3px 0" }}
                >
                  → /{page}
                </a>
              ))}
            </div>

            <div style={{ display: "flex", gap: 6 }}>
              <button data-testid="copy-url" onClick={copyUrl} style={{ ...primaryButton, flex: 1 }}>Copy URL</button>
              <button
                data-testid="delete-button"
                onClick={() => selectedInfo && requestDelete(selectedInfo.filename)}
                style={{
                  flex: 1,
                  background: "none",
                  border: "1px solid rgba(255,100,100,0.4)",
                  color: "#ff6b6b",
                  borderRadius: 4,
                  padding: "6px 10px",
                  cursor: "pointer",
                  fontSize: 11,
                }}
              >
                Delete
              </button>
            </div>
          </>
        ) : (
          <div style={{ fontSize: 11, color: "rgba(255,255,255,0.3)" }}>Select an image</div>
        )}
      </div>
    </div>
  );
}

function RailRow({
  active,
  onClick,
  testid,
  children,
}: {
  active: boolean;
  onClick: () => void;
  testid?: string;
  children: React.ReactNode;
}) {
  return (
    <div
      data-testid={testid}
      onClick={onClick}
      style={{
        cursor: "pointer",
        padding: "3px 8px",
        borderRadius: 3,
        marginBottom: 3,
        background: active ? "rgba(51,172,183,0.1)" : "transparent",
        color: active ? "var(--accent)" : "rgba(255,255,255,0.7)",
        fontSize: 12,
      }}
    >
      {children}
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
  return { position: "absolute", top: 4, right: 4, background: bg, color: "#000", fontSize: 10, padding: "1px 5px", borderRadius: 2 };
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
const sectionLabel: React.CSSProperties = {
  fontSize: 10,
  color: "rgba(255,255,255,0.45)",
  textTransform: "uppercase",
  letterSpacing: "0.05em",
  margin: "14px 0 6px",
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
  overflow: "auto",
};
const primaryButton: React.CSSProperties = {
  background: "var(--accent)",
  border: "none",
  borderRadius: 4,
  padding: "6px 10px",
  color: "#000",
  cursor: "pointer",
  fontSize: 11,
  fontWeight: "bold",
};
const dropzoneStyle: React.CSSProperties = {
  marginTop: 18,
  border: "1px dashed rgba(255,255,255,0.2)",
  borderRadius: 4,
  padding: 14,
  textAlign: "center",
  color: "rgba(255,255,255,0.5)",
  fontSize: 11,
  cursor: "pointer",
  background: "rgba(255,255,255,0.02)",
};
const modalBackdrop: React.CSSProperties = {
  position: "fixed", inset: 0, background: "rgba(0,0,0,0.55)",
  display: "flex", alignItems: "center", justifyContent: "center", zIndex: 9999,
};
const modalBoxStyle: React.CSSProperties = {
  background: "var(--background)",
  border: "1px solid rgba(255,255,255,0.15)",
  borderRadius: 6,
  padding: 18,
  width: 380,
};
const cancelButton: React.CSSProperties = {
  background: "none",
  border: "1px solid rgba(255,255,255,0.15)",
  color: "rgba(255,255,255,0.7)",
  padding: "5px 12px",
  borderRadius: 4,
  cursor: "pointer",
  fontSize: 11,
};
const dangerButton: React.CSSProperties = {
  background: "#ff6b6b",
  border: "none",
  color: "#000",
  padding: "5px 12px",
  borderRadius: 4,
  cursor: "pointer",
  fontSize: 11,
  fontWeight: "bold",
};
