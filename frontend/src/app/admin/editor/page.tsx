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
  published: boolean;
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
  const [createdAt, setCreatedAt] = useState("");
  const [published, setPublished] = useState(true);
  const [saving, setSaving] = useState(false);
  const [aiPrompt, setAiPrompt] = useState("");
  const [aiLoading, setAiLoading] = useState(false);
  const [aiMessages, setAiMessages] = useState<{ role: "user" | "assistant"; text: string; contentStatus?: "generating" | "done" }[]>([]);
  const [aiPanelOpen, setAiPanelOpen] = useState(false);
  const aiChatEndRef = useRef<HTMLDivElement>(null);
  const [editorWidth, setEditorWidth] = useState(50); // percentage of editor area
  const [aiPanelWidth, setAiPanelWidth] = useState(320); // pixels
  const [toast, setToast] = useState<string | null>(null);
  const [message, setMessage] = useState("");
  const [showNewPage, setShowNewPage] = useState(false);
  const [showNewFolder, setShowNewFolder] = useState(false);
  const [newName, setNewName] = useState("");
  const [newTitle, setNewTitle] = useState("");
  const [newFolder, setNewFolder] = useState("");
  const [newFolderParent, setNewFolderParent] = useState("");
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const [preAiContent, setPreAiContent] = useState<string | null>(null);
  const [contentSource, setContentSource] = useState<"user" | "ai" | "load">("load");
  const autoSaveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const preAiContentRef = useRef<string | null>(null);

  const loadTree = useCallback(async () => {
    const res = await apiFetch("/api/pages/tree?drafts=true");
    if (res.ok) {
      const data = await res.json();
      if (Array.isArray(data)) setTree(data);
    }
  }, []);

  useEffect(() => {
    loadTree();
  }, [loadTree]);

  // Keep ref in sync with state to avoid stale closures in SSE handler
  useEffect(() => { preAiContentRef.current = preAiContent; }, [preAiContent]);

  // Auto-save: only for user-initiated changes, debounced
  useEffect(() => {
    if (contentSource !== "user") return;
    if (!selectedPath) return;

    if (autoSaveTimerRef.current) {
      clearTimeout(autoSaveTimerRef.current);
    }

    autoSaveTimerRef.current = setTimeout(async () => {
      const res = await apiFetch(`/api/pages/${selectedPath}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ title, content, show_date: showDate, published, created_at: createdAt }),
      });
      setMessage(res.ok ? "Auto-saved" : "Auto-save failed");
      if (res.ok) loadTree();
      setTimeout(() => setMessage(""), 2000);
    }, 2500);

    return () => {
      if (autoSaveTimerRef.current) {
        clearTimeout(autoSaveTimerRef.current);
      }
    };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [content, title, showDate, published, createdAt, contentSource, selectedPath]);

  const loadPage = async (path: string) => {
    if (autoSaveTimerRef.current) {
      clearTimeout(autoSaveTimerRef.current);
      autoSaveTimerRef.current = null;
    }
    const res = await apiFetch(`/api/pages/${path}`);
    if (res.ok) {
      const data: PageData = await res.json();
      setSelectedPath(path);
      setTitle(data.title);
      setContent(data.content);
      setShowDate(data.show_date);
      setPublished(data.published);
      setCreatedAt(data.created_at);
      setMeta({ view_count: data.view_count, created_at: data.created_at });
      setMessage("");
      setContentSource("load");
      setPreAiContent(null);
      preAiContentRef.current = null;
    }
  };

  const savePage = async () => {
    if (!selectedPath) return;
    setSaving(true);
    const res = await apiFetch(`/api/pages/${selectedPath}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ title, content, show_date: showDate, published, created_at: createdAt }),
    });
    setSaving(false);
    setMessage(res.ok ? "Saved" : "Save failed");
    if (res.ok) loadTree();
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

  const showToast = (msg: string) => {
    setToast(msg);
    setTimeout(() => setToast(null), 5000);
  };

  const scrollAiChat = () => {
    setTimeout(() => aiChatEndRef.current?.scrollIntoView({ behavior: "smooth" }), 0);
  };

  const runAiPrompt = async () => {
    if (!aiPrompt.trim() || aiLoading) return;
    const userMsg = aiPrompt.trim();
    setAiLoading(true);
    setAiPanelOpen(true);

    // Add user message to chat
    const userEntry = { role: "user" as const, text: userMsg };
    setAiMessages((prev) => [...prev, userEntry]);
    setAiPrompt("");
    scrollAiChat();

    // Build history for API (include content edit markers)
    const history = [...aiMessages].map((m) => ({
      role: m.role,
      text: m.text,
      ...(m.contentStatus === "done" ? { content_edit: true } : {}),
    })).filter((m) => m.text || m.content_edit);

    const token = localStorage.getItem("access_token");
    try {
      const res = await fetch("/api/ai/complete", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({ prompt: userMsg, content, history }),
      });

      const reader = res.body?.getReader();
      if (!reader) {
        showToast("Streaming not supported");
        setAiLoading(false);
        return;
      }

      // Add empty assistant message to stream into
      const assistantIdx = aiMessages.length + 1; // +1 for the user message we just added
      setAiMessages((prev) => [...prev, { role: "assistant", text: "" }]);

      const decoder = new TextDecoder();
      let accumulatedText = "";
      let buffer = "";
      let contentIndicatorAdded = false;

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split("\n");
        buffer = lines.pop() || "";

        for (const line of lines) {
          if (!line.startsWith("data: ")) continue;
          const data = line.slice(6);
          if (data === "[DONE]") continue;

          try {
            const parsed = JSON.parse(data);
            if (parsed.error) {
              showToast(parsed.error);
              setAiMessages((prev) => {
                const updated = [...prev];
                updated[assistantIdx] = { role: "assistant", text: "Error: " + parsed.error };
                return updated;
              });
              setAiLoading(false);
              return;
            }
            if (parsed.type === "text" && parsed.text) {
              accumulatedText += parsed.text;
              const text = accumulatedText;
              setAiMessages((prev) => {
                const updated = [...prev];
                updated[assistantIdx] = { role: "assistant", text };
                return updated;
              });
              scrollAiChat();
            }
            if (parsed.type === "content_start") {
              contentIndicatorAdded = true;
              setAiMessages((prev) => [...prev, { role: "assistant", text: "", contentStatus: "generating" }]);
              scrollAiChat();
            }
            if (parsed.type === "content" && parsed.content) {
              setContent((prev) => {
                if (preAiContentRef.current === null) {
                  setPreAiContent(prev);
                  preAiContentRef.current = prev;
                }
                return parsed.content;
              });
              setContentSource("ai");
              if (contentIndicatorAdded) {
                setAiMessages((prev) => {
                  const updated = [...prev];
                  const idx = updated.findLastIndex((m) => m.contentStatus === "generating");
                  if (idx >= 0) updated[idx] = { ...updated[idx], contentStatus: "done" };
                  return updated;
                });
              } else {
                setAiMessages((prev) => [...prev, { role: "assistant", text: "", contentStatus: "done" }]);
              }
              scrollAiChat();
            }
          } catch {
            // skip unparseable lines
          }
        }
      }
    } catch {
      showToast("AI request failed — check your connection");
    }
    setAiLoading(false);
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
        setContentSource("user");
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
    setContentSource("user");
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
          <span style={{ display: "flex", gap: "10px", alignItems: "center" }}>
            <a
              href="/"
              style={{
                color: "rgba(255,255,255,0.4)",
                textDecoration: "none",
                fontFamily: "inherit",
                fontSize: "0.75rem",
                cursor: "pointer",
              }}
            >
              site
            </a>
            <a
              href="/admin/settings"
              style={{
                color: "rgba(255,255,255,0.4)",
                textDecoration: "none",
                fontFamily: "inherit",
                fontSize: "0.75rem",
                cursor: "pointer",
              }}
            >
              settings
            </a>
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
          </span>
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
              <EditablePath
                path={selectedPath}
                onRename={async (newPath) => {
                  const res = await apiFetch(`/api/pages/${selectedPath}`, {
                    method: "PATCH",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({ new_path: newPath }),
                  });
                  if (res.ok) {
                    setSelectedPath(newPath);
                    loadTree();
                    return null;
                  }
                  const data = await res.json().catch(() => ({ error: "Rename failed" }));
                  return data.error || "Rename failed";
                }}
              />
              <input
                value={title}
                onChange={(e) => { setTitle(e.target.value); setContentSource("user"); }}
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
              <span style={{ flex: 1 }} />
              <button
                onClick={() => setAiPanelOpen(!aiPanelOpen)}
                style={{
                  background: aiPanelOpen ? "rgba(51,172,183,0.2)" : "rgba(255,255,255,0.05)",
                  border: `1px solid ${aiPanelOpen ? "var(--accent)" : "rgba(255,255,255,0.1)"}`,
                  borderRadius: "3px",
                  color: aiPanelOpen ? "var(--accent)" : "var(--color)",
                  padding: "3px 8px",
                  cursor: "pointer",
                  fontFamily: "inherit",
                  fontSize: "0.75rem",
                }}
              >
                AI
              </button>
            </div>

            {/* Editor + Preview + AI Panel */}
            <div style={{ flex: 1, display: "flex", overflow: "hidden" }}>
              {/* AI Panel (collapsible, left side) */}
              {aiPanelOpen && (
                <div
                  style={{
                    width: `${aiPanelWidth}px`,
                    display: "flex",
                    flexDirection: "column",
                    flexShrink: 0,
                    background: "rgba(255,255,255,0.02)",
                  }}
                >
                  {/* AI Panel Header */}
                  <div
                    style={{
                      padding: "8px 12px",
                      borderBottom: "1px solid rgba(255,255,255,0.1)",
                      display: "flex",
                      justifyContent: "space-between",
                      alignItems: "center",
                    }}
                  >
                    <span style={{ color: "var(--accent)", fontSize: "0.75rem", fontWeight: "bold" }}>
                      AI Assistant
                    </span>
                    <button
                      onClick={() => setAiPanelOpen(false)}
                      style={{
                        background: "none",
                        border: "none",
                        color: "rgba(255,255,255,0.4)",
                        cursor: "pointer",
                        fontFamily: "inherit",
                        fontSize: "0.9rem",
                        padding: "0 4px",
                      }}
                    >
                      ✕
                    </button>
                  </div>

                  {/* Chat Messages */}
                  <div
                    style={{
                      flex: 1,
                      overflow: "auto",
                      padding: "12px",
                      display: "flex",
                      flexDirection: "column",
                      gap: "10px",
                    }}
                  >
                    {aiMessages.length === 0 && (
                      <span style={{ color: "rgba(255,255,255,0.3)", fontSize: "0.8rem" }}>
                        Ask me to edit your content, translate, summarize, or answer questions...
                      </span>
                    )}
                    {aiMessages.map((msg, i) => (
                      <div
                        key={i}
                        style={{
                          alignSelf: msg.role === "user" ? "flex-end" : "flex-start",
                          maxWidth: "85%",
                          padding: "8px 12px",
                          borderRadius: msg.role === "user" ? "12px 12px 2px 12px" : "12px 12px 12px 2px",
                          background: msg.role === "user"
                            ? "rgba(51,172,183,0.2)"
                            : "rgba(255,255,255,0.06)",
                          color: msg.role === "user"
                            ? "var(--accent)"
                            : "rgba(255,255,255,0.8)",
                          fontSize: "0.8rem",
                          lineHeight: "1.5",
                          whiteSpace: "pre-wrap",
                          wordBreak: "break-word",
                        }}
                      >
                        {msg.contentStatus ? (
                          <span style={{
                            display: "inline-flex",
                            alignItems: "center",
                            gap: "6px",
                            fontStyle: "italic",
                            color: msg.contentStatus === "generating" ? "var(--accent)" : "rgba(255,255,255,0.4)",
                            fontSize: "0.75rem",
                          }}>
                            {msg.contentStatus === "generating" ? (
                              <><span style={{ animation: "pulse 1.5s infinite", display: "inline-block", width: 6, height: 6, borderRadius: "50%", background: "var(--accent)" }} /> Updating content...</>
                            ) : (
                              <><span style={{ color: "var(--accent)" }}>&#10003;</span> Content updated</>
                            )}
                          </span>
                        ) : msg.text || (msg.role === "assistant" && aiLoading ? "..." : "")}
                      </div>
                    ))}
                    <div ref={aiChatEndRef} />
                  </div>

                  {/* AI Input Area */}
                  <div
                    style={{
                      padding: "8px 12px",
                      borderTop: "1px solid rgba(255,255,255,0.1)",
                      display: "flex",
                      gap: "6px",
                    }}
                  >
                    <input
                      value={aiPrompt}
                      onChange={(e) => setAiPrompt(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === "Enter" && !e.shiftKey) {
                          e.preventDefault();
                          runAiPrompt();
                        }
                      }}
                      placeholder="Ask AI..."
                      disabled={aiLoading}
                      style={{
                        flex: 1,
                        background: "var(--background)",
                        border: "1px solid rgba(255,255,255,0.15)",
                        borderRadius: "4px",
                        padding: "6px 10px",
                        color: "var(--color)",
                        fontFamily: "inherit",
                        fontSize: "0.8rem",
                        outline: "none",
                      }}
                    />
                    <button
                      onClick={runAiPrompt}
                      disabled={aiLoading || !aiPrompt.trim()}
                      style={{
                        background: aiLoading ? "rgba(51,172,183,0.3)" : "var(--accent)",
                        border: "none",
                        borderRadius: "4px",
                        padding: "6px 12px",
                        color: "#000",
                        fontFamily: "inherit",
                        fontWeight: "bold",
                        cursor: aiLoading ? "wait" : "pointer",
                        fontSize: "0.75rem",
                        flexShrink: 0,
                      }}
                    >
                      {aiLoading ? "..." : "→"}
                    </button>
                  </div>
                </div>
              )}
              {aiPanelOpen && (
                <DragHandle
                  onDrag={(delta) => setAiPanelWidth((w) => Math.max(200, Math.min(600, w + delta)))}
                />
              )}
              <div style={{ width: `${editorWidth}%`, flexShrink: 0, flexGrow: 0, display: "flex", flexDirection: "column" }}>
                {preAiContent !== null && (
                  <div style={{
                    padding: "6px 16px",
                    background: "rgba(51, 172, 183, 0.08)",
                    borderBottom: "1px solid rgba(51, 172, 183, 0.2)",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "space-between",
                    fontSize: "0.8rem",
                  }}>
                    <span style={{ color: "rgba(255,255,255,0.5)" }}>
                      AI edited this content
                    </span>
                    <div style={{ display: "flex", gap: "8px" }}>
                      <button
                        onClick={() => {
                          setContent(preAiContent);
                          setPreAiContent(null);
                          preAiContentRef.current = null;
                          setContentSource("user");
                        }}
                        style={{
                          background: "none",
                          border: "1px solid rgba(255, 100, 100, 0.4)",
                          borderRadius: "3px",
                          padding: "2px 10px",
                          color: "#ff6b6b",
                          cursor: "pointer",
                          fontFamily: "inherit",
                          fontSize: "0.75rem",
                        }}
                      >
                        Revert
                      </button>
                      <button
                        onClick={() => { setPreAiContent(null); preAiContentRef.current = null; }}
                        style={{
                          background: "none",
                          border: "1px solid rgba(255,255,255,0.15)",
                          borderRadius: "3px",
                          padding: "2px 10px",
                          color: "rgba(255,255,255,0.5)",
                          cursor: "pointer",
                          fontFamily: "inherit",
                          fontSize: "0.75rem",
                        }}
                      >
                        Dismiss
                      </button>
                    </div>
                  </div>
                )}
                <textarea
                  ref={textareaRef}
                  value={content}
                  onChange={(e) => { setContent(e.target.value); setContentSource("user"); }}
                  style={{
                    flex: 1,
                    background: "var(--background)",
                    color: "var(--color)",
                    border: "none",
                    padding: "16px",
                    fontFamily: "inherit",
                    fontSize: "0.85rem",
                    lineHeight: "1.6",
                    resize: "none",
                    outline: "none",
                  }}
                />
              </div>
              <DragHandle
                onDrag={(delta, containerWidth) => {
                  setEditorWidth((w) => {
                    const pct = (delta / containerWidth) * 100;
                    return Math.max(20, Math.min(80, w + pct));
                  });
                }}
              />
              <div
                className="editor-preview"
                style={{
                  flex: 1,
                  overflow: "auto",
                  padding: "0 16px",
                  minWidth: 0,
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
                  <label style={{ color: "rgba(255,255,255,0.4)", display: "flex", alignItems: "center", gap: "4px" }}>
                    Created:
                    <input
                      type="date"
                      value={createdAt ? createdAt.slice(0, 10) : ""}
                      onChange={(e) => {
                        const date = e.target.value;
                        if (date) {
                          setCreatedAt(date + "T00:00:00Z");
                          setContentSource("user");
                        }
                      }}
                      style={{
                        background: "transparent",
                        border: "none",
                        color: "rgba(255,255,255,0.4)",
                        fontFamily: "inherit",
                        fontSize: "inherit",
                        outline: "none",
                        cursor: "pointer",
                      }}
                    />
                  </label>
                  <span style={{ color: "rgba(255,255,255,0.4)" }}>
                    Views: {meta.view_count}
                  </span>
                  <label style={{ color: "rgba(255,255,255,0.4)", display: "flex", alignItems: "center", gap: "4px", cursor: "pointer" }}>
                    <input
                      type="checkbox"
                      checked={showDate}
                      onChange={(e) => { setShowDate(e.target.checked); setContentSource("user"); }}
                      style={{ accentColor: "var(--accent)" }}
                    />
                    Show date
                  </label>
                  <label style={{ color: "rgba(255,255,255,0.4)", display: "flex", alignItems: "center", gap: "4px", cursor: "pointer" }}>
                    <input
                      type="checkbox"
                      checked={published}
                      onChange={(e) => { setPublished(e.target.checked); setContentSource("user"); }}
                      style={{ accentColor: "var(--accent)" }}
                    />
                    Published
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

      {/* Error Toast */}
      {toast && (
        <div
          style={{
            position: "fixed",
            bottom: "20px",
            right: "20px",
            maxWidth: "400px",
            background: "rgba(40, 30, 30, 0.95)",
            border: "1px solid rgba(255, 100, 100, 0.4)",
            borderRadius: "8px",
            padding: "12px 16px",
            color: "#ff6b6b",
            fontSize: "0.8rem",
            fontFamily: "inherit",
            lineHeight: "1.5",
            zIndex: 9999,
            boxShadow: "0 4px 20px rgba(0, 0, 0, 0.4)",
            cursor: "pointer",
            wordBreak: "break-word",
          }}
          onClick={() => setToast(null)}
        >
          {toast}
        </div>
      )}
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

function EditablePath({
  path,
  onRename,
}: {
  path: string;
  onRename: (newPath: string) => Promise<string | null>;
}) {
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState(path);
  const [error, setError] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => { setValue(path); setEditing(false); setError(null); }, [path]);
  useEffect(() => { if (editing) inputRef.current?.select(); }, [editing]);

  const submit = async () => {
    const trimmed = value.trim();
    if (!trimmed || trimmed === path) {
      setEditing(false);
      setValue(path);
      setError(null);
      return;
    }
    const err = await onRename(trimmed);
    if (err) {
      setError(err);
    } else {
      setEditing(false);
      setError(null);
    }
  };

  if (!editing) {
    return (
      <span
        onClick={() => setEditing(true)}
        title="Click to rename"
        style={{
          color: "rgba(255,255,255,0.4)",
          fontSize: "0.8rem",
          flexShrink: 0,
          cursor: "pointer",
          display: "flex",
          alignItems: "center",
          gap: "4px",
        }}
      >
        {path}
        <Pencil size={11} style={{ opacity: 0.5 }} />
      </span>
    );
  }

  return (
    <span style={{ flexShrink: 0, display: "flex", alignItems: "center", gap: "6px" }}>
      <input
        ref={inputRef}
        value={value}
        onChange={(e) => { setValue(e.target.value); setError(null); }}
        onKeyDown={(e) => {
          if (e.key === "Enter") submit();
          if (e.key === "Escape") { setEditing(false); setValue(path); setError(null); }
        }}
        onBlur={submit}
        style={{
          background: "var(--background)",
          border: `1px solid ${error ? "rgba(255,100,100,0.5)" : "rgba(255,255,255,0.2)"}`,
          borderRadius: "3px",
          padding: "2px 6px",
          color: "var(--color)",
          fontFamily: "inherit",
          fontSize: "0.8rem",
          outline: "none",
          width: `${Math.max(value.length, 10)}ch`,
        }}
      />
      {error && <span style={{ color: "#ff6b6b", fontSize: "0.75rem" }}>{error}</span>}
    </span>
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

function DragHandle({
  onDrag,
}: {
  onDrag: (deltaX: number, containerWidth: number) => void;
}) {
  const handleRef = useRef<HTMLDivElement>(null);

  const onMouseDown = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault();
      const startX = e.clientX;
      const container = handleRef.current?.parentElement;
      const containerWidth = container?.clientWidth || 1;

      const onMouseMove = (e: MouseEvent) => {
        const delta = e.clientX - startX;
        onDrag(delta, containerWidth);
        // Reset startX so delta is incremental
        (onMouseMove as any)._startX = e.clientX;
      };

      // Use incremental deltas
      let lastX = startX;
      const onMove = (e: MouseEvent) => {
        const delta = e.clientX - lastX;
        lastX = e.clientX;
        onDrag(delta, containerWidth);
      };

      const onMouseUp = () => {
        document.removeEventListener("mousemove", onMove);
        document.removeEventListener("mouseup", onMouseUp);
        document.body.style.cursor = "";
        document.body.style.userSelect = "";
      };

      document.addEventListener("mousemove", onMove);
      document.addEventListener("mouseup", onMouseUp);
      document.body.style.cursor = "col-resize";
      document.body.style.userSelect = "none";
    },
    [onDrag]
  );

  return (
    <div
      ref={handleRef}
      onMouseDown={onMouseDown}
      style={{
        width: "5px",
        cursor: "col-resize",
        flexShrink: 0,
        background: "rgba(255,255,255,0.06)",
        transition: "background 0.15s",
      }}
      onMouseEnter={(e) => (e.currentTarget.style.background = "rgba(51,172,183,0.3)")}
      onMouseLeave={(e) => (e.currentTarget.style.background = "rgba(255,255,255,0.06)")}
    />
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
