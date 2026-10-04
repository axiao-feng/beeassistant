import { Download, Menu, Share2 } from "lucide-react";
import { useEffect, useState } from "react";
import { appActions } from "@/app/store";
import { useAppDispatch, useAppSelector } from "@/app/hooks";
import { Button } from "@/components/ui/button";
import { AuthExpiredDialog } from "@/features/auth/AuthExpiredDialog";
import { shortID } from "@/lib/format";
import type { ChatViewMessage } from "@/types/chat";
import { Sidebar } from "./Sidebar";
import { SessionShareDialog } from "./SessionShareDialog";

export function AppShell({ children }: { children: React.ReactNode }) {
  const dispatch = useAppDispatch();
  const toast = useAppSelector((state) => state.app.toast);
  const authExpired = useAppSelector((state) => state.app.authExpired);
  const [shareTarget, setShareTarget] = useState<{ session_id: string; title?: string } | null>(null);
  const activePanel = useAppSelector((state) => state.app.activePanel);
  const activeSessionID = useAppSelector((state) => state.chat.activeSessionID);
  const messages = useAppSelector((state) => state.chat.messages);
  const sessions = useAppSelector((state) => state.sessions.items);
  const title = resolveTitle(activePanel, activeSessionID, sessions);
  const activeSession = sessions.find((item) => item.session_id === activeSessionID);
  const canShareSession = activePanel === "chat" && Boolean(activeSessionID);
  const canExportSession = canShareSession && messages.some((message) => !message.hidden && message.content.trim());

  useEffect(() => {
    if (!toast) return;
    const timer = window.setTimeout(() => dispatch(appActions.showToast(undefined)), 2000);
    return () => window.clearTimeout(timer);
  }, [dispatch, toast]);

  useEffect(() => {
    if (!canShareSession) setShareTarget(null);
  }, [canShareSession]);

  return (
    <div className="flex h-[var(--app-viewport-height,100dvh)] overflow-hidden bg-background/95 text-foreground">
      <div className="contents" inert={authExpired ? true : undefined}>
        <Sidebar />
        <main className="relative flex min-w-0 flex-1 flex-col">
          <header className="sketch-rule flex h-14 shrink-0 items-center justify-between border-b bg-background/82 px-3 backdrop-blur sm:px-5">
            <div className="flex min-w-0 items-center gap-3">
              <Button
                className="md:hidden"
                size="icon"
                variant="ghost"
                aria-label="打开导航"
                onClick={() => dispatch(appActions.setSidebarOpen(true))}
              >
                <Menu className="h-4 w-4" />
              </Button>
              <div className="min-w-0 truncate text-base font-semibold">
                {title}
              </div>
            </div>
            {canShareSession ? (
              <div className="flex shrink-0 items-center gap-1">
                {canExportSession ? (
                  <button
                    className="flex h-9 w-9 items-center justify-center text-muted-foreground transition-colors hover:text-foreground"
                    aria-label="导出会话"
                    title="导出会话 Markdown"
                    type="button"
                    onClick={() => exportSessionMarkdown(activeSessionID, activeSession?.title, messages, dispatch)}
                  >
                    <Download className="h-4 w-4" />
                  </button>
                ) : null}
                <button
                  className="flex h-9 w-9 items-center justify-center text-muted-foreground transition-colors hover:text-foreground"
                  aria-label="分享会话"
                  title="分享会话"
                  type="button"
                  onClick={() => setShareTarget({ session_id: activeSessionID, title: activeSession?.title })}
                >
                  <Share2 className="h-4 w-4" />
                </button>
              </div>
            ) : null}
          </header>
          <div className="min-h-0 flex-1 overflow-hidden">{children}</div>
        </main>
        <SessionShareDialog
          session={shareTarget}
          onClose={() => setShareTarget(null)}
        />
        {toast ? (
          <div className="sketch-surface fixed bottom-4 left-4 right-4 z-50 rounded-md px-4 py-3 text-sm sm:left-auto sm:w-auto">
            {toast}
          </div>
        ) : null}
      </div>
      <AuthExpiredDialog open={authExpired} />
    </div>
  );
}

function exportSessionMarkdown(
  sessionID: string,
  title: string | undefined,
  messages: ChatViewMessage[],
  dispatch: ReturnType<typeof useAppDispatch>,
) {
  const name = title?.trim() || shortID(sessionID) || "会话";
  const content = messages
    .filter((message) => !message.hidden && message.content.trim())
    .map((message) => {
      const role = message.role === "user" ? "用户" : message.role === "assistant" ? "助手" : message.role === "tool" ? "工具" : "系统";
      const agent = message.agent ? `（${message.agent}）` : "";
      const time = message.createdAt ? `\n\n> ${message.createdAt}` : "";
      return `## ${role}${agent}${time}\n\n${message.content.trim()}`;
    })
    .join("\n\n---\n\n");
  const markdown = `# ${name}\n\n> 会话 ID：${sessionID}\n\n${content}\n`;
  const blob = new Blob([markdown], { type: "text/markdown;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = `beeteams-${safeFileName(name)}-${new Date().toISOString().slice(0, 10)}.md`;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
  dispatch(appActions.showToast("会话已导出"));
}

function safeFileName(value: string) {
  return value.replace(/[<>:"/\\|?*\u0000-\u001f]/g, "_").trim().slice(0, 50) || "session";
}

function resolveTitle(
  activePanel: "chat" | "config" | "files" | "schedules" | "shares" | "skills",
  activeSessionID: string,
  sessions: Array<{ session_id: string; title?: string }>,
) {
  if (activePanel !== "chat") {
    return {
      files: "文件与知识库",
      schedules: "待办与日程",
      shares: "分享",
      skills: "技能与自动化",
      config: "偏好与记忆",
    }[activePanel];
  }
  if (!activeSessionID) return "新会话";
  const session = sessions.find((item) => item.session_id === activeSessionID);
  return session?.title || shortID(activeSessionID) || "新会话";
}
