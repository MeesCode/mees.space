export interface TreeNode {
  name: string;
  path: string;
  title?: string;
  is_dir: boolean;
  children?: TreeNode[];
  show_date?: boolean;
  created_at?: string;
  published?: boolean;
}

export interface PageData {
  path: string;
  title: string;
  content: string;
  view_count: number;
  created_at: string;
  updated_at: string;
  show_date: boolean;
  published: boolean;
}

export interface ImageInfo {
  filename: string;
  url: string;
  size: number;
  ref_count: number;     // -1 means refs scan failed; treat as unknown
  uploaded_at: string;   // RFC3339
}

export interface ImageRefs {
  filename: string;
  pages: string[];       // page paths (no .md, no leading slash)
}
