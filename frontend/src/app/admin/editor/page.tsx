"use client";

import { useEffect, useState, useRef, useCallback } from "react";
import { apiFetch } from "@/lib/api";
import { logout } from "@/lib/auth";
import { MarkdownRenderer } from "@/components/MarkdownRenderer";

interface TreeNode {
  name: string;
  path: string;
  title?: string;
  is_dir: boolean;
  children?: TreeNode[];
}

interface PageData {
  path: string;
  title: string;
  content: string;
  view_count: number;
  created_at: string;
  updated_at: string;
}

interface ImageInfo {
  filename: string;
  url: string;
  size: number;
}

export default function EditorPage() {
  const [tree, setTree] = useState<TreeNode[]>([]);
  const [selectedPath, setSelectedPath] = useState<string | null>(null);
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const [meta, setMeta] = useState<{
    view_count: number;
    created_at: string;
  } | null>(null);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
  const [showNewPage, setShowNewPage] = useState(false);
  const [showNewFolder, setShowNewFolder] = useState(false);
  const [newName, setNewName] = useState("");
  const [newTitle, setNewTitle] = useState("");
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const loadTree = useCallback(async () => {
    const res = await apiFetch("/api/pages/tree");
    if (res.ok) {
      const data = await res.json();
      if (Array.isArray(data)) setTree(data);
    }
  }, []);

  useEffect(() => {
    loadTree();
  }, [loadTree]);

  const loadPage = async (path: string) => {
    const res = await apiFetch(`/api/pages/${path}`);
    if (res.ok) {
      const data: PageData = await res.json();
      setSelectedPath(path);
      setTitle(data.title);
      setContent(data.content);
      setMeta({ view_count: data.view_count, created_at: data.created_at });
      setMessage("");
    }
  };

  const savePage = async () => {
    if (!selectedPath) return;
    setSaving(true);
    const res = await apiFetch(`/api/pages/${selectedPath}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ title, content }),
    });
    setSaving(false);
    setMessage(res.ok ? "Saved" : "Save failed");
    setTimeout(() => setMessage(""), 2000);
  };

  const deletePage = async () => {
    if (!selectedPath || !confirm(`Delete "${selectedPath}"?`)) return;
    const res = await apiFetch(`/api/pages/${selectedPath}`, {
      method: "DELETE",
    });
    if (res.ok) {
      setSelectedPath(null);
      setTitle("");
      setContent("");
      setMeta(null);
      loadTree();
    }
  };

  const createPage = async () => {
    if (!newName || !newTitle) return;
    const path = selectedPath
      ? tree.find(
          (n) => n.is_dir && (n.path === selectedPath || selectedPath?.startsWith(n.path + "/"))
        )
        ? selectedPath.includes("/")
          ? selectedPath.substring(0, selectedPath.lastIndexOf("/")) + "/" + newName
          : newName
        : newName
      : newName;

    const res = await apiFetch(`/api/pages/${path}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ title: newTitle, content: "" }),
    });
    if (res.ok) {
      setShowNewPage(false);
      setNewName("");
      setNewTitle("");
      loadTree();
      loadPage(path);
    }
  };

  const createFolder = async () => {
    if (!newName) return;
    const res = await apiFetch(`/api/folders/${newName}`, { method: "POST" });
    if (res.ok) {
      setShowNewFolder(false);
      setNewName("");
      loadTree();
    }
  };

  const uploadImage = async (file: File) => {
    const form = new FormData();
    form.append("file", file);
    const res = await apiFetch("/api/images", { method: "POST", body: form });
    if (res.ok) {
      const data: ImageInfo = await res.json();
      const ta = textareaRef.current;
      if (ta) {
        const pos = ta.selectionStart;
        const md = `![${file.name}](${data.url})`;
        setContent((prev) => prev.slice(0, pos) + md + prev.slice(pos));
      }
    }
  };

  const insertMarkdown = (before: string, after: string = "") => {
    const ta = textareaRef.current;
    if (!ta) return;
    const start = ta.selectionStart;
    const end = ta.selectionEnd;
    const selected = content.slice(start, end);
    const replacement = before + selected + after;
    setContent((prev) => prev.slice(0, start) + replacement + prev.slice(end));
    setTimeout(() => {
      ta.focus();
      ta.setSelectionRange(
        start + before.length,
        start + before.length + selected.length
      );
    }, 0);
  };

  const toolbarButtons = [
    { label: "B", action: () => insertMarkdown("**", "**"), title: "Bold" },
    { label: "I", action: () => insertMarkdown("*", "*"), title: "Italic" },
    { label: "H1", action: () => insertMarkdown("# "), title: "Heading 1" },
    { label: "H2", action: () => insertMarkdown("## "), title: "Heading 2" },
    {
      label: "Link",
      action: () => insertMarkdown("[", "](url)"),
      title: "Link",
    },
    {
      label: "Img",
      action: () => insertMarkdown("![alt](", ")"),
      title: "Image",
    },
    {
      label: "Code",
      action: () => insertMarkdown("`", "`"),
      title: "Inline code",
    },
    {
      label: "```",
      action: () => insertMarkdown("```\n", "\n```"),
      title: "Code block",
    },
    {
      label: "List",
      action: () => insertMarkdown("- "),
      title: "Bullet list",
    },
    {
      label: "1.",
      action: () => insertMarkdown("1. "),
      title: "Numbered list",
    },
  ];

  return (
    <div style={{ display: "flex", height: "100vh", overflow: "hidden" }}>
      {/* File Tree Panel */}
      <div
        style={{
          width: "260px",
          borderRight: "1px solid rgba(255,255,255,0.1)",
          padding: "16px",
          overflow: "auto",
          flexShrink: 0,
        }}
      >
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            marginBottom: "16px",
          }}
        >
          <span
            style={{
              color: "var(--accent)",
              fontWeight: "bold",
              fontSize: "0.9rem",
            }}
          >
            pages
          </span>
          <button
            onClick={logout}
            style={{
              background: "none",
              border: "none",
              color: "rgba(255,255,255,0.4)",
              cursor: "pointer",
              fontFamily: "inherit",
              fontSize: "0.75rem",
            }}
          >
            logout
          </button>
        </div>

        <FileTree
          nodes={tree}
          selectedPath={selectedPath}
          onSelect={loadPage}
        />

        <div
          style={{
            marginTop: "16px",
            display: "flex",
            gap: "8px",
            flexWrap: "wrap",
          }}
        >
          <SmallButton onClick={() => setShowNewPage(true)}>
            + page
          </SmallButton>
          <SmallButton onClick={() => setShowNewFolder(true)}>
            + folder
          </SmallButton>
        </div>

        {showNewPage && (
          <div style={{ marginTop: "12px" }}>
            <input
              placeholder="path (e.g. blogs/my-post)"
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              style={inputStyle}
            />
            <input
              placeholder="title"
              value={newTitle}
              onChange={(e) => setNewTitle(e.target.value)}
              style={{ ...inputStyle, marginTop: "6px" }}
            />
            <div style={{ display: "flex", gap: "6px", marginTop: "6px" }}>
              <SmallButton onClick={createPage}>create</SmallButton>
              <SmallButton
                onClick={() => {
                  setShowNewPage(false);
                  setNewName("");
                  setNewTitle("");
                }}
              >
                cancel
              </SmallButton>
            </div>
          </div>
        )}

        {showNewFolder && (
          <div style={{ marginTop: "12px" }}>
            <input
              placeholder="folder path"
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              style={inputStyle}
            />
            <div style={{ display: "flex", gap: "6px", marginTop: "6px" }}>
              <SmallButton onClick={createFolder}>create</SmallButton>
              <SmallButton
                onClick={() => {
                  setShowNewFolder(false);
                  setNewName("");
                }}
              >
                cancel
              </SmallButton>
            </div>
          </div>
        )}
      </div>

      {/* Editor Panel */}
      <div style={{ flex: 1, display: "flex", flexDirection: "column", overflow: "hidden" }}>
        {selectedPath ? (
          <>
            {/* Title bar */}
            <div
              style={{
                padding: "12px 16px",
                borderBottom: "1px solid rgba(255,255,255,0.1)",
                display: "flex",
                alignItems: "center",
                gap: "12px",
              }}
            >
              <span
                style={{
                  color: "rgba(255,255,255,0.4)",
                  fontSize: "0.8rem",
                  flexShrink: 0,
                }}
              >
                {selectedPath}
              </span>
              <input
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder="Page title"
                style={{
                  ...inputStyle,
                  flex: 1,
                  fontWeight: "bold",
                  fontSize: "1rem",
                }}
              />
            </div>

            {/* Toolbar */}
            <div
              style={{
                padding: "6px 16px",
                borderBottom: "1px solid rgba(255,255,255,0.1)",
                display: "flex",
                gap: "4px",
                flexWrap: "wrap",
                alignItems: "center",
              }}
            >
              {toolbarButtons.map((btn) => (
                <button
                  key={btn.label}
                  onClick={btn.action}
                  title={btn.title}
                  style={{
                    background: "rgba(255,255,255,0.05)",
                    border: "1px solid rgba(255,255,255,0.1)",
                    borderRadius: "3px",
                    color: "var(--color)",
                    padding: "3px 8px",
                    cursor: "pointer",
                    fontFamily: "inherit",
                    fontSize: "0.75rem",
                  }}
                >
                  {btn.label}
                </button>
              ))}
              <label
                style={{
                  background: "rgba(255,255,255,0.05)",
                  border: "1px solid rgba(255,255,255,0.1)",
                  borderRadius: "3px",
                  color: "var(--color)",
                  padding: "3px 8px",
                  cursor: "pointer",
                  fontSize: "0.75rem",
                }}
              >
                Upload
                <input
                  type="file"
                  accept="image/*"
                  style={{ display: "none" }}
                  onChange={(e) => {
                    const file = e.target.files?.[0];
                    if (file) uploadImage(file);
                    e.target.value = "";
                  }}
                />
              </label>
            </div>

            {/* Editor + Preview */}
            <div style={{ flex: 1, display: "flex", overflow: "hidden" }}>
              <textarea
                ref={textareaRef}
                value={content}
                onChange={(e) => setContent(e.target.value)}
                style={{
                  flex: 1,
                  background: "var(--background)",
                  color: "var(--color)",
                  border: "none",
                  borderRight: "1px solid rgba(255,255,255,0.1)",
                  padding: "16px",
                  fontFamily: "inherit",
                  fontSize: "0.85rem",
                  lineHeight: "1.6",
                  resize: "none",
                  outline: "none",
                }}
              />
              <div
                style={{
                  flex: 1,
                  overflow: "auto",
                  padding: "0 16px",
                }}
              >
                <MarkdownRenderer content={content} />
              </div>
            </div>

            {/* Bottom bar */}
            <div
              style={{
                padding: "8px 16px",
                borderTop: "1px solid rgba(255,255,255,0.1)",
                display: "flex",
                alignItems: "center",
                gap: "16px",
                fontSize: "0.8rem",
              }}
            >
              {meta && (
                <>
                  <span style={{ color: "rgba(255,255,255,0.4)" }}>
                    Created: {new Date(meta.created_at).toLocaleDateString()}
                  </span>
                  <span style={{ color: "rgba(255,255,255,0.4)" }}>
                    Views: {meta.view_count}
                  </span>
                </>
              )}
              <span style={{ flex: 1 }} />
              {message && (
                <span style={{ color: "var(--accent)" }}>{message}</span>
              )}
              <button
                onClick={savePage}
                disabled={saving}
                style={{
                  background: "var(--accent)",
                  border: "none",
                  borderRadius: "4px",
                  padding: "6px 16px",
                  color: "#000",
                  fontFamily: "inherit",
                  fontWeight: "bold",
                  cursor: saving ? "wait" : "pointer",
                  fontSize: "0.85rem",
                }}
              >
                {saving ? "Saving..." : "Save"}
              </button>
              <button
                onClick={deletePage}
                style={{
                  background: "none",
                  border: "1px solid rgba(255,100,100,0.4)",
                  borderRadius: "4px",
                  padding: "6px 16px",
                  color: "#ff6b6b",
                  fontFamily: "inherit",
                  cursor: "pointer",
                  fontSize: "0.85rem",
                }}
              >
                Delete
              </button>
            </div>
          </>
        ) : (
          <div
            style={{
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              height: "100%",
              color: "rgba(255,255,255,0.3)",
            }}
          >
            Select a page to edit
          </div>
        )}
      </div>
    </div>
  );
}

function FileTree({
  nodes,
  selectedPath,
  onSelect,
  depth = 0,
}: {
  nodes: TreeNode[];
  selectedPath: string | null;
  onSelect: (path: string) => void;
  depth?: number;
}) {
  return (
    <div style={{ paddingLeft: depth > 0 ? 12 : 0 }}>
      {nodes.map((node) => {
        if (node.is_dir) {
          return (
            <FolderNode
              key={node.path}
              node={node}
              selectedPath={selectedPath}
              onSelect={onSelect}
              depth={depth}
            />
          );
        }
        return (
          <div
            key={node.path}
            onClick={() => onSelect(node.path)}
            style={{
              padding: "3px 8px",
              borderRadius: "3px",
              cursor: "pointer",
              fontSize: "0.8rem",
              color:
                selectedPath === node.path
                  ? "var(--accent)"
                  : "rgba(255,255,255,0.7)",
              backgroundColor:
                selectedPath === node.path
                  ? "rgba(51, 172, 183, 0.1)"
                  : "transparent",
            }}
          >
            {node.title || node.name}
          </div>
        );
      })}
    </div>
  );
}

function FolderNode({
  node,
  selectedPath,
  onSelect,
  depth,
}: {
  node: TreeNode;
  selectedPath: string | null;
  onSelect: (path: string) => void;
  depth: number;
}) {
  const [open, setOpen] = useState(true);

  return (
    <div>
      <div
        onClick={() => setOpen(!open)}
        style={{
          padding: "3px 8px",
          cursor: "pointer",
          fontSize: "0.75rem",
          color: "rgba(255,255,255,0.4)",
          textTransform: "uppercase",
          letterSpacing: "0.05em",
          userSelect: "none",
        }}
      >
        {open ? "▾" : "▸"} {node.name}
      </div>
      {open && node.children && (
        <FileTree
          nodes={node.children}
          selectedPath={selectedPath}
          onSelect={onSelect}
          depth={depth + 1}
        />
      )}
    </div>
  );
}

function SmallButton({
  onClick,
  children,
}: {
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      onClick={onClick}
      style={{
        background: "rgba(255,255,255,0.05)",
        border: "1px solid rgba(255,255,255,0.15)",
        borderRadius: "3px",
        padding: "4px 10px",
        color: "var(--color)",
        fontFamily: "inherit",
        fontSize: "0.75rem",
        cursor: "pointer",
      }}
    >
      {children}
    </button>
  );
}

const inputStyle: React.CSSProperties = {
  width: "100%",
  background: "var(--background)",
  border: "1px solid rgba(255,255,255,0.15)",
  borderRadius: "4px",
  padding: "6px 10px",
  color: "var(--color)",
  fontFamily: "inherit",
  fontSize: "0.85rem",
  outline: "none",
};
