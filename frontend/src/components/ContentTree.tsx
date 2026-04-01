"use client";

import { useEffect, useState } from "react";
import { useNavigation } from "@/lib/navigation";

interface TreeNode {
  name: string;
  path: string;
  title?: string;
  is_dir: boolean;
  children?: TreeNode[];
  show_date?: boolean;
  created_at?: string;
}

function sortNodes(nodes: TreeNode[]): TreeNode[] {
  const files = nodes.filter((n) => !n.is_dir);
  const dirs = nodes.filter((n) => n.is_dir);
  files.sort((a, b) => a.name.localeCompare(b.name));
  dirs.sort((a, b) => a.name.localeCompare(b.name));
  return [...files, ...dirs];
}

export function ContentTree() {
  const [tree, setTree] = useState<TreeNode[]>([]);
  const { path, navigate } = useNavigation();

  useEffect(() => {
    fetch("/api/pages/tree")
      .then((r) => r.json())
      .then((data) => {
        if (Array.isArray(data)) setTree(data);
      })
      .catch(() => {});
  }, []);

  if (tree.length === 0) return null;

  const currentPath =
    path === "/" ? "home" : path.replace(/^\//, "");

  return (
    <nav className="content-nav">
      {sortNodes(tree).map((node) => (
        <TreeNodeItem key={node.path} node={node} currentPath={currentPath} navigate={navigate} />
      ))}
    </nav>
  );
}

function TreeNodeItem({
  node,
  currentPath,
  navigate,
}: {
  node: TreeNode;
  currentPath: string;
  navigate: (href: string) => void;
}) {
  const [open, setOpen] = useState(true);

  if (node.is_dir) {
    return (
      <div>
        <div className="folder-name" onClick={() => setOpen(!open)}>
          {open ? "▾" : "▸"} {node.name}
        </div>
        {open && node.children && (
          <div className="folder-children">
            {sortNodes(node.children).map((child) => (
              <TreeNodeItem
                key={child.path}
                node={child}
                currentPath={currentPath}
                navigate={navigate}
              />
            ))}
          </div>
        )}
      </div>
    );
  }

  const href = node.path === "home" ? "/" : `/${node.path}`;
  const isActive = currentPath === node.path;

  const dateStr = node.show_date && node.created_at
    ? new Date(node.created_at).toLocaleDateString("en-CA")
    : null;

  return (
    <a
      href={href}
      className={isActive ? "active" : ""}
      onClick={(e) => {
        e.preventDefault();
        navigate(href);
      }}
    >
      {node.title || node.name}
      {dateStr && (
        <span style={{ marginLeft: "8px", fontSize: "0.75em", opacity: 0.5 }}>
          {dateStr}
        </span>
      )}
    </a>
  );
}
