export interface MemoryEntry {
  id: string;
  type: string;
  summary: string;
  detail?: string;
  tags?: string[];
  session_id?: string;
  created_at?: string;
  hit_count?: number;
  last_hit_at?: string;
}
