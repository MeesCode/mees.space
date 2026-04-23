"use client";

import { PageData } from "@/lib/types";

interface Props {
  page: PageData;
}

export function PageMeta({ page }: Props) {
  if (!page.show_date) return null;

  const date = new Date(page.created_at).toLocaleDateString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
  });

  const words = page.content.trim().split(/\s+/).filter(Boolean).length;
  const minutes = Math.max(1, Math.ceil(words / 200));

  const viewLabel = page.view_count === 1 ? "1 view" : `${page.view_count} views`;

  return (
    <div className="page-meta">
      <span>{date}</span>
      <span className="sep">·</span>
      <span>{minutes} min read</span>
      <span className="sep">·</span>
      <span>{viewLabel}</span>
    </div>
  );
}
