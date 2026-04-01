"use client";

import { ContentTree } from "./ContentTree";
import { isLoggedIn } from "@/lib/auth";
import { useEffect, useState } from "react";

export function Sidebar() {
  const [loggedIn, setLoggedIn] = useState(false);

  useEffect(() => {
    setLoggedIn(isLoggedIn());
  }, []);

  return (
    <aside className="app-header">
      <div style={{ position: "relative", display: "inline-block" }}>
        <div className="app-header-avatar-border glow" />
        <div className="app-header-avatar-border alternate" />
        <div className="app-header-avatar-border" />
        <img
          className="app-header-avatar"
          src="/mees.png"
          alt="Mees Brinkhuis"
        />
      </div>
      <h1>Mees Brinkhuis</h1>
      <div className="app-header-social">
        <a
          target="_blank"
          href="https://www.linkedin.com/in/mees-brinkhuis/"
          rel="noreferrer noopener"
        >
          <img className="icon" src="/linkedin.svg" alt="linkedin" />
        </a>
        <a
          target="_blank"
          href="https://github.com/MeesCode"
          rel="noreferrer noopener"
        >
          <img className="icon" src="/github.svg" alt="github" />
        </a>
      </div>
      <ContentTree />
      {loggedIn && (
        <a
          href="/admin/editor"
          style={{
            display: "block",
            marginTop: "24px",
            color: "rgba(255,255,255,0.3)",
            textDecoration: "none",
            fontSize: "0.75rem",
          }}
        >
          admin →
        </a>
      )}
    </aside>
  );
}
