"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";

export default function LoginPage() {
  const router = useRouter();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);

    try {
      const res = await fetch("/api/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username, password }),
      });

      if (!res.ok) {
        const data = await res.json();
        setError(data.error || "Login failed");
        setLoading(false);
        return;
      }

      const data = await res.json();
      localStorage.setItem("access_token", data.access_token);
      localStorage.setItem("refresh_token", data.refresh_token);
      router.push("/admin/editor");
    } catch {
      setError("Connection failed");
      setLoading(false);
    }
  };

  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        minHeight: "100vh",
        padding: "20px",
      }}
    >
      <form
        onSubmit={handleSubmit}
        style={{
          width: "100%",
          maxWidth: "360px",
          display: "flex",
          flexDirection: "column",
          gap: "16px",
        }}
      >
        <h1 className="text-accent" style={{ fontSize: "1.2rem", margin: 0 }}>
          admin login
        </h1>

        {error && (
          <div style={{ color: "#ff6b6b", fontSize: "0.85rem" }}>{error}</div>
        )}

        <input
          type="text"
          placeholder="username"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          autoComplete="username"
          className="form-input"
          style={{ padding: "10px 14px", fontSize: "0.9rem" }}
        />

        <input
          type="password"
          placeholder="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          autoComplete="current-password"
          className="form-input"
          style={{ padding: "10px 14px", fontSize: "0.9rem" }}
        />

        <button
          type="submit"
          disabled={loading}
          className="btn btn-primary"
          style={{ padding: "10px 14px", fontSize: "0.9rem" }}
        >
          {loading ? "logging in..." : "log in"}
        </button>
      </form>
    </div>
  );
}
