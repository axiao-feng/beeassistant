export interface FileEntry {
  name: string;
  path: string;
  is_dir?: boolean;
  size?: number;
  mod_time?: string;
}

export interface FileContent {
  name?: string;
  path: string;
  content: string;
  size?: number;
  mod_time?: string | number;
}

export interface ContentSearchResult {
  name: string;
  path: string;
  line: number;
  text: string;
  size?: number;
  mod_time?: number;
}

export interface PreviewLink {
  id?: string;
  link_id?: string;
  path?: string;
  file_path?: string;
  file_paths?: string[];
  has_password?: boolean;
  url?: string;
  created_at?: string | number;
  expires_at?: string | number;
}
