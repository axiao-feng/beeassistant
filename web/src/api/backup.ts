import { request } from "./client";

export async function downloadBackup() {
  const response = await fetch("/api/fkteams/backup/export", { credentials: "same-origin" });
  if (!response.ok) {
    throw new Error(`备份导出失败（${response.status}）`);
  }
  const blob = await response.blob();
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = `beeteams-backup-${new Date().toISOString().slice(0, 10)}.zip`;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

export function restoreBackup(file: File) {
  const body = new FormData();
  body.append("backup", file);
  return request<{ restored: number; restart_required: boolean }>("/api/fkteams/backup/restore", {
    method: "POST",
    body,
  });
}
