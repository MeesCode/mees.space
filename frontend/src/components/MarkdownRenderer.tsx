"use client";

import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeRaw from "rehype-raw";
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
        rehypePlugins={[rehypeRaw, rehypeHighlight]}
        components={{
          img: (props) => <img {...props} loading="lazy" decoding="async" />,
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}
