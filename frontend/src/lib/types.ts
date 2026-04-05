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
}
