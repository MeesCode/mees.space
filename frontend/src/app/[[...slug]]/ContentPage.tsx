"use client";

import { useEffect, useRef, useState } from "react";
import { useNavigation } from "@/lib/navigation";
import { MarkdownRenderer } from "@/components/MarkdownRenderer";
import { PageMeta } from "@/components/PageMeta";
import { TerminalPrompt } from "@/components/TerminalPrompt";
import { PageData } from "@/lib/types";

interface BootstrapPage extends PageData {
  rendered_html?: string;
}

// Parsed bootstrap is cached at module load so StrictMode's double-invocation
// of the useState initializer returns the same object both times.
// `undefined` = not yet read; `null` = read, but missing or invalid.
let bootstrapCache: BootstrapPage | null | undefined = undefined;

function readBootstrap(): BootstrapPage | null {
  if (bootstrapCache !== undefined) return bootstrapCache;
  if (typeof document === "undefined") return null;
  const el = document.getElementById("__page_data__");
  if (!el || !el.textContent) {
    bootstrapCache = null;
    return null;
  }
  try {
    bootstrapCache = JSON.parse(el.textContent) as BootstrapPage;
    return bootstrapCache;
  } catch (err) {
    console.warn("failed to parse __page_data__", err);
    bootstrapCache = null;
    return null;
  }
}

function applyScroll() {
  if (typeof window === "undefined") return;
  const hash = window.location.hash.slice(1);
  if (hash) {
    const target = document.getElementById(hash);
    if (target) {
      target.scrollIntoView();
      return;
    }
  }
  window.scrollTo(0, 0);
}

export function ContentPage() {
  const { path } = useNavigation();
  const pagePath = path === "/" ? "home" : path.replace(/^\//, "");

  const [page, setPage] = useState<PageData | null>(() => {
    const boot = readBootstrap();
    if (boot && boot.path === pagePath) {
      return boot;
    }
    return null;
  });
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(page === null);
  const ssrHTMLRef = useRef<string | null>(
    (page as BootstrapPage | null)?.rendered_html ?? null
  );
  const usedSSR = ssrHTMLRef.current !== null;
  const firstRunRef = useRef(true);

  useEffect(() => {
    if (pagePath.startsWith("admin")) return;

    // First render used the bootstrap — skip the initial fetch but still
    // fire view count and run the scroll + highlight pass against the
    // server-rendered DOM.
    if (firstRunRef.current && usedSSR && page) {
      firstRunRef.current = false;
      applyScroll();
      import("highlight.js").then(({ default: hljs }) => {
        document
          .querySelectorAll<HTMLElement>("#content pre code")
          .forEach((el) => hljs.highlightElement(el));
      });
      fetch(`/api/views/${pagePath}`, { method: "POST" }).catch(() => {});
      return;
    }

    firstRunRef.current = false;
    ssrHTMLRef.current = null;
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
        applyScroll();
        fetch(`/api/views/${pagePath}`, { method: "POST" }).catch(() => {});
      })
      .catch(() => {
        setError("Page not found");
        setLoading(false);
      });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pagePath]);

  if (pagePath.startsWith("admin")) {
    return null;
  }

  if (loading) {
    return (
      <>
        <div
          id="content"
          suppressHydrationWarning
          dangerouslySetInnerHTML={{ __html: "<!--SSR_CONTENT-->" }}
        />
        <TerminalPrompt path={pagePath} />
      </>
    );
  }

  if (error || !page) {
    const messages = [
      "This page is on a coffee break. Indefinitely. ☕",
      "404: page not found. But you found this message, so that's something.",
      "The bits that were here have been recycled into something better.",
      "You've reached the edge of the internet. Turn back.",
      "This page moved out and didn't leave a forwarding address.",
    ];
    const message = messages[Math.floor(Math.random() * messages.length)];

    return (
      <>
        <article id="content">
          <h1>404</h1>
          <p>{message}</p>
          <p style={{ marginTop: "24px" }}>
            <a href="/" onClick={(e) => { e.preventDefault(); window.location.href = "/"; }}>
              ← take me home
            </a>
          </p>
        </article>
        <TerminalPrompt path={pagePath} />
      </>
    );
  }

  return (
    <>
      <PageMeta page={page} />
      <div className={page.show_date ? "has-meta" : ""}>
        {usedSSR && ssrHTMLRef.current !== null ? (
          <div
            id="content"
            dangerouslySetInnerHTML={{ __html: ssrHTMLRef.current }}
          />
        ) : (
          <MarkdownRenderer content={page.content} />
        )}
      </div>
      <TerminalPrompt path={page.path} />
    </>
  );
}
