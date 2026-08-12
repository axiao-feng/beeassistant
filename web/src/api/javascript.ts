import { post } from "@/api/client";
import type { JavaScriptToolConfig } from "@/types/config";

export interface JavaScriptToolTestResponse {
  result: unknown;
  raw: string;
  notices: Array<{ level: string; message: string }>;
}

export function testJavaScriptTool(tool: JavaScriptToolConfig, input: Record<string, unknown>) {
  return post<JavaScriptToolTestResponse>("/api/fkteams/javascript/test", { tool, input });
}
