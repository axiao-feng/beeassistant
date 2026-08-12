import { describe, expect, test } from "bun:test";
import { isMCPToolGroup, splitToolCatalog } from "@/features/config/toolCatalog";
import type { ToolInfo } from "@/types/config";

describe("tool catalog classification", () => {
  test("keeps custom tools separate from MCP tools", () => {
    const tools: ToolInfo[] = [
      { name: "file", builtin: true },
      { name: "javascript", builtin: false, category: "扩展" },
      { name: "mcp-demo", builtin: false, category: "MCP" },
    ];

    const groups = splitToolCatalog(tools);
    expect(groups.builtin.map((tool) => tool.name)).toEqual(["file"]);
    expect(groups.custom.map((tool) => tool.name)).toEqual(["javascript"]);
    expect(groups.mcp.map((tool) => tool.name)).toEqual(["mcp-demo"]);
  });

  test("recognizes prefixed MCP groups when category metadata is absent", () => {
    expect(isMCPToolGroup({ name: "mcp-external", builtin: false })).toBe(true);
  });
});
