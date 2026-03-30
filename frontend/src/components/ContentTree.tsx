"use client";

import { useEffect, useState } from "react";
import { usePathname } from "next/navigation";
import Link from "next/link";

interface TreeNode {
  name: string;
  path: string;
  title?: string;
  is_dir: boolean;
  children?: TreeNode[];
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
  const pathname = usePathname();

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
    pathname === "/" ? "home" : pathname.replace(/^\//, "");

  return (
    <nav className="content-nav">
      {sortNodes(tree).map((node) => (
        <TreeNodeItem key={node.path} node={node} currentPath={currentPath} />
      ))}
    </nav>
  );
}

function TreeNodeItem({
  node,
  currentPath,
}: {
  node: TreeNode;
  currentPath: string;
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
              />
            ))}
          </div>
        )}
      </div>
    );
  }

  const href = node.path === "home" ? "/" : `/${node.path}`;
  const isActive = currentPath === node.path;

  return (
    <Link href={href} className={isActive ? "active" : ""}>
      {node.title || node.name}
    </Link>
  );
}
