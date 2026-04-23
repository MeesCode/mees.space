"use client";

import { useEffect, useState } from "react";
import { apiFetch } from "@/lib/api";

export default function SettingsPage() {
  const [systemPrompt, setSystemPrompt] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [model, setModel] = useState("claude-sonnet-4-6");
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");

  useEffect(() => {
    apiFetch("/api/settings")
      .then((r) => r.json())
      .then((data) => {
        setSystemPrompt(data.ai_system_prompt || "");
        setModel(data.ai_model || "claude-sonnet-4-6");
        setApiKey("");
      });
  }, []);

  const save = async () => {
    setSaving(true);
    const body: Record<string, string> = { ai_system_prompt: systemPrompt, ai_model: model };
    if (apiKey) {
      body.ai_api_key = apiKey;
    }
    const res = await apiFetch("/api/settings", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    setSaving(false);
    setMessage(res.ok ? "Saved" : "Save failed");
    if (res.ok) setApiKey("");
    setTimeout(() => setMessage(""), 2000);
  };

  return (
    <div
      style={{
        maxWidth: "700px",
        margin: "0 auto",
        padding: "40px 20px",
      }}
    >
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          marginBottom: "32px",
        }}
      >
        <h1
          style={{
            color: "var(--accent)",
            fontSize: "1.2rem",
            margin: 0,
          }}
        >
          settings
        </h1>
        <a
          href="/admin/editor"
          style={{
            color: "rgba(255,255,255,0.4)",
            textDecoration: "none",
            fontSize: "0.85rem",
            fontFamily: "inherit",
          }}
        >
          ← back to editor
        </a>
      </div>

      <div style={{ marginBottom: "24px" }}>
        <label
          style={{
            display: "block",
            color: "rgba(255,255,255,0.6)",
            fontSize: "0.8rem",
            marginBottom: "8px",
            textTransform: "uppercase",
            letterSpacing: "0.05em",
          }}
        >
          AI System Prompt
        </label>
        <textarea
          value={systemPrompt}
          onChange={(e) => setSystemPrompt(e.target.value)}
          rows={6}
          style={{
            width: "100%",
            background: "var(--background)",
            border: "1px solid rgba(255,255,255,0.15)",
            borderRadius: "4px",
            padding: "10px",
            color: "var(--color)",
            fontFamily: "inherit",
            fontSize: "0.85rem",
            lineHeight: "1.6",
            resize: "vertical",
            outline: "none",
          }}
        />
        <p
          style={{
            color: "rgba(255,255,255,0.3)",
            fontSize: "0.75rem",
            marginTop: "6px",
          }}
        >
          This is prepended as a system message to every AI request from the
          editor.
        </p>
      </div>

      <div style={{ marginBottom: "24px" }}>
        <label
          style={{
            display: "block",
            color: "rgba(255,255,255,0.6)",
            fontSize: "0.8rem",
            marginBottom: "8px",
            textTransform: "uppercase",
            letterSpacing: "0.05em",
          }}
        >
          Model
        </label>
        <select
          value={model}
          onChange={(e) => setModel(e.target.value)}
          style={{
            width: "100%",
            background: "var(--background)",
            border: "1px solid rgba(255,255,255,0.15)",
            borderRadius: "4px",
            padding: "8px 28px 8px 10px",
            color: "var(--color)",
            fontFamily: "inherit",
            fontSize: "0.85rem",
            outline: "none",
            appearance: "auto" as const,
          }}
        >
          <option value="claude-haiku-4-5">Claude 4.5 Haiku (fast, cheap)</option>
          <option value="claude-sonnet-4-6">Claude 4.6 Sonnet (balanced)</option>
          <option value="claude-opus-4-7">Claude 4.7 Opus (smartest)</option>
        </select>
      </div>

      <div style={{ marginBottom: "32px" }}>
        <label
          style={{
            display: "block",
            color: "rgba(255,255,255,0.6)",
            fontSize: "0.8rem",
            marginBottom: "8px",
            textTransform: "uppercase",
            letterSpacing: "0.05em",
          }}
        >
          Anthropic API Key
        </label>
        <input
          type="password"
          value={apiKey}
          onChange={(e) => setApiKey(e.target.value)}
          placeholder="sk-ant-••••••••"
          style={{
            width: "100%",
            background: "var(--background)",
            border: "1px solid rgba(255,255,255,0.15)",
            borderRadius: "4px",
            padding: "8px 10px",
            color: "var(--color)",
            fontFamily: "inherit",
            fontSize: "0.85rem",
            outline: "none",
          }}
        />
        <p
          style={{
            color: "rgba(255,255,255,0.3)",
            fontSize: "0.75rem",
            marginTop: "6px",
          }}
        >
          Leave empty to keep the current key. The key is stored on the server
          and never sent to the browser.
        </p>
      </div>

      <div style={{ display: "flex", alignItems: "center", gap: "12px" }}>
        <button
          onClick={save}
          disabled={saving}
          style={{
            background: "var(--accent)",
            border: "none",
            borderRadius: "4px",
            padding: "8px 20px",
            color: "#000",
            fontFamily: "inherit",
            fontWeight: "bold",
            cursor: saving ? "wait" : "pointer",
            fontSize: "0.85rem",
          }}
        >
          {saving ? "Saving..." : "Save"}
        </button>
        {message && (
          <span style={{ color: "var(--accent)", fontSize: "0.85rem" }}>
            {message}
          </span>
        )}
      </div>
    </div>
  );
}
