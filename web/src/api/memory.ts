import type { MemoryEntry } from "@/types/memory";
import { del, post } from "./client";
import { get } from "./client";

export function listMemories() {
  return get<MemoryEntry[]>("/api/fkteams/memory");
}

export function deleteMemory(summary: string) {
  return del<{ deleted: number }>("/api/fkteams/memory", { summary });
}

export function clearMemories() {
  return post<{ cleared: number }>("/api/fkteams/memory/clear");
}
