"use client";

import { useEffect, useState, useRef, useCallback } from "react";
import { apiFetch } from "@/lib/api";
import { logout } from "@/lib/auth";
import { MarkdownRenderer } from "@/components/MarkdownRenderer";
import { Pencil, Trash2 } from "lucide-react";

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
  show_date: boolean;
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
  const [showDate, setShowDate] = useState(false);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
  const [showNewPage, setShowNewPage] = useState(false);
  const [showNewFolder, setShowNewFolder] = useState(false);
  const [newName, setNewName] = useState("");
  const [newTitle, setNewTitle] = useState("");
  const [newFolder, setNewFolder] = useState("");
  const [newFolderParent, setNewFolderParent] = useState("");
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
      setShowDate(data.show_date);
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
      body: JSON.stringify({ title, content, show_date: showDate }),
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

  const flattenFolders = (nodes: TreeNode[], prefix = ""): string[] => {
    const folders: string[] = [];
    for (const node of nodes) {
      if (node.is_dir) {
        folders.push(node.path);
        if (node.children) {
          folders.push(...flattenFolders(node.children, node.path + "/"));
        }
      }
    }
    return folders;
  };

  const createPage = async () => {
    if (!newName || !newTitle) return;
    const path = newFolder ? newFolder + "/" + newName : newName;

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

  const renameFolder = async (path: string, newName: string) => {
    const res = await apiFetch(`/api/folders/${path}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: newName }),
    });
    if (res.ok) {
      loadTree();
    } else {
      const data = await res.json().catch(() => ({ error: "Rename failed" }));
      alert(data.error || "Rename failed");
    }
  };

  const deleteFolder = async (path: string) => {
    if (!confirm(`Delete folder "${path}" and all its contents?`)) return;
    const res = await apiFetch(`/api/folders/${path}?recursive=true`, {
      method: "DELETE",
    });
    if (res.ok) {
      loadTree();
    } else {
      const data = await res.json().catch(() => ({ error: "Delete failed" }));
      alert(data.error || "Delete failed");
    }
  };

  const createFolder = async () => {
    if (!newName) return;
    const path = newFolderParent ? newFolderParent + "/" + newName : newName;
    const res = await apiFetch(`/api/folders/${path}`, { method: "POST" });
    if (res.ok) {
      setShowNewFolder(false);
      setNewName("");
      setNewFolderParent("");
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
          onRenameFolder={renameFolder}
          onDeleteFolder={deleteFolder}
        />

        <div
          style={{
            marginTop: "16px",
            display: "flex",
            gap: "8px",
            flexWrap: "wrap",
          }}
        >
          <SmallButton onClick={() => { setShowNewPage(!showNewPage); setShowNewFolder(false); setNewName(""); setNewTitle(""); setNewFolder(""); }}>
            + page
          </SmallButton>
          <SmallButton onClick={() => { setShowNewFolder(!showNewFolder); setShowNewPage(false); setNewName(""); setNewFolderParent(""); }}>
            + folder
          </SmallButton>
        </div>

        {showNewPage && (
          <div style={{ marginTop: "12px" }}>
            <select
              value={newFolder}
              onChange={(e) => setNewFolder(e.target.value)}
              style={{ ...inputStyle, ...selectStyle, marginBottom: "6px" }}
            >
              <option value="">(root)</option>
              {flattenFolders(tree).map((f) => (
                <option key={f} value={f}>{f}</option>
              ))}
            </select>
            <input
              placeholder="slug (e.g. my-post)"
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
                  setNewFolder("");
                }}
              >
                cancel
              </SmallButton>
            </div>
          </div>
        )}

        {showNewFolder && (
          <div style={{ marginTop: "12px" }}>
            <select
              value={newFolderParent}
              onChange={(e) => setNewFolderParent(e.target.value)}
              style={{ ...inputStyle, ...selectStyle, marginBottom: "6px" }}
            >
              <option value="">(root)</option>
              {flattenFolders(tree).map((f) => (
                <option key={f} value={f}>{f}</option>
              ))}
            </select>
            <input
              placeholder="folder name"
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
                  setNewFolderParent("");
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
                  <label style={{ color: "rgba(255,255,255,0.4)", display: "flex", alignItems: "center", gap: "4px", cursor: "pointer" }}>
                    <input
                      type="checkbox"
                      checked={showDate}
                      onChange={(e) => setShowDate(e.target.checked)}
                      style={{ accentColor: "var(--accent)" }}
                    />
                    Show date
                  </label>
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
  onRenameFolder,
  onDeleteFolder,
}: {
  nodes: TreeNode[];
  selectedPath: string | null;
  onSelect: (path: string) => void;
  depth?: number;
  onRenameFolder: (path: string, newName: string) => void;
  onDeleteFolder: (path: string) => void;
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
              onRename={onRenameFolder}
              onDelete={onDeleteFolder}
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
  onRename,
  onDelete,
}: {
  node: TreeNode;
  selectedPath: string | null;
  onSelect: (path: string) => void;
  depth: number;
  onRename: (path: string, newName: string) => void;
  onDelete: (path: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [hovered, setHovered] = useState(false);
  const [renaming, setRenaming] = useState(false);
  const [renameName, setRenameName] = useState(node.name);

  const handleRenameSubmit = () => {
    if (renameName && renameName !== node.name) {
      onRename(node.path, renameName);
    }
    setRenaming(false);
  };

  return (
    <div>
      <div
        onMouseEnter={() => setHovered(true)}
        onMouseLeave={() => setHovered(false)}
        style={{
          padding: "3px 8px",
          cursor: "pointer",
          fontSize: "0.75rem",
          color: "rgba(255,255,255,0.4)",
          textTransform: "uppercase",
          letterSpacing: "0.05em",
          userSelect: "none",
          display: "flex",
          alignItems: "center",
          gap: "4px",
        }}
      >
        <span onClick={() => setOpen(!open)} style={{ flex: 1 }}>
          {open ? "▾" : "▸"}{" "}
          {renaming ? (
            <input
              value={renameName}
              onChange={(e) => setRenameName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") handleRenameSubmit();
                if (e.key === "Escape") { setRenaming(false); setRenameName(node.name); }
              }}
              onBlur={handleRenameSubmit}
              autoFocus
              onClick={(e) => e.stopPropagation()}
              style={{
                background: "var(--background)",
                border: "1px solid rgba(255,255,255,0.3)",
                borderRadius: "2px",
                color: "var(--color)",
                fontFamily: "inherit",
                fontSize: "inherit",
                textTransform: "none" as const,
                padding: "1px 4px",
                width: "80%",
              }}
            />
          ) : (
            node.name
          )}
        </span>
        {hovered && !renaming && (
          <span style={{ display: "flex", gap: "2px" }}>
            <Pencil
              size={12}
              onClick={(e) => { e.stopPropagation(); setRenaming(true); setRenameName(node.name); }}
              style={{ cursor: "pointer" }}
            />
            <Trash2
              size={12}
              onClick={(e) => { e.stopPropagation(); onDelete(node.path); }}
              style={{ cursor: "pointer", color: "#ff6b6b" }}
            />
          </span>
        )}
      </div>
      {open && node.children && (
        <FileTree
          nodes={node.children}
          selectedPath={selectedPath}
          onSelect={onSelect}
          depth={depth + 1}
          onRenameFolder={onRename}
          onDeleteFolder={onDelete}
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

const selectStyle: React.CSSProperties = {
  paddingRight: "28px",
  appearance: "auto",
};
