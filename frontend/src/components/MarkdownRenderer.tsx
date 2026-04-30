"use client";

import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeRaw from "rehype-raw";
import rehypeSlug from "rehype-slug";
import rehypeAutolinkHeadings from "rehype-autolink-headings";
import rehypeHighlight from "rehype-highlight";
import "highlight.js/styles/atom-one-dark.css";

interface Props {
  content: string;
}

export function MarkdownRenderer({ content }: Props) {
  return (
    <div id="content">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[
          rehypeRaw,
          rehypeSlug,
          [
            rehypeAutolinkHeadings,
            {
              behavior: "append",
              properties: {
                className: ["heading-anchor"],
                "aria-hidden": "true",
                tabIndex: -1,
              },
              content: { type: "text", value: "#" },
            },
          ],
          rehypeHighlight,
        ]}
        components={{
          img: (props) => <img {...props} loading="lazy" decoding="async" />,
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}
