"use client";

import { useEffect, useState } from "react";
import { useNavigation } from "@/lib/navigation";
import { MarkdownRenderer } from "@/components/MarkdownRenderer";
import { TerminalPrompt } from "@/components/TerminalPrompt";

interface PageData {
  path: string;
  title: string;
  content: string;
  view_count: number;
  created_at: string;
  updated_at: string;
}

export function ContentPage() {
  const { path } = useNavigation();
  const [page, setPage] = useState<PageData | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const pagePath = path === "/" ? "home" : path.replace(/^\//, "");

  useEffect(() => {
    // Don't fetch for admin routes
    if (pagePath.startsWith("admin")) return;

    setLoading(true);
    setError(null);

    fetch(`/api/pages/${pagePath}`)
      .then((r) => {
        if (!r.ok) throw new Error("not found");
        return r.json();
      })
      .then((data) => {
        setPage(data);
        setLoading(false);
        window.scrollTo(0, 0);

        // Increment view count
        fetch(`/api/views/${pagePath}`, { method: "POST" }).catch(
          () => {}
        );
      })
      .catch(() => {
        setError("Page not found");
        setLoading(false);
      });
  }, [pagePath]);

  if (pagePath.startsWith("admin")) {
    return null;
  }

  if (loading) {
    return (
      <>
        <article id="content">
          <p style={{ opacity: 0.5 }}>Loading...</p>
        </article>
        <TerminalPrompt path={pagePath} />
      </>
    );
  }

  if (error || !page) {
    return (
      <>
        <article id="content">
          <h1>404</h1>
          <p>Page not found.</p>
        </article>
        <TerminalPrompt path={pagePath} />
      </>
    );
  }

  return (
    <>
      <MarkdownRenderer content={page.content} />
      <TerminalPrompt path={page.path} />
    </>
  );
}
