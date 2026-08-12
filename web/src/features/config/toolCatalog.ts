import type { ToolInfo } from "@/types/config";

export function isMCPToolGroup(tool: ToolInfo) {
  return tool.builtin === false && (tool.category === "MCP" || tool.name.startsWith("mcp-"));
}

export function splitToolCatalog(tools: ToolInfo[]) {
  return {
    builtin: tools.filter((tool) => tool.builtin !== false),
    custom: tools.filter((tool) => tool.builtin === false && !isMCPToolGroup(tool)),
    mcp: tools.filter(isMCPToolGroup),
  };
}
