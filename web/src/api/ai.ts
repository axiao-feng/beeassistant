import { post } from "@/api/client";
import type { AgentConfig } from "@/types/config";
import type { JavaScriptHookConfig, JavaScriptToolConfig } from "@/types/config";
import type { SkillDraft } from "@/types/skills";

export interface AgentDraftRequest {
  instruction: string;
  existing_agents?: string[];
  available_tools?: string[];
  available_models?: string[];
  default_model_id?: string;
}

export interface AgentDraftResponse {
  agents: AgentConfig[];
}

export interface RewriteTextRequest {
  scenario: string;
  instruction: string;
  text: string;
  context?: Record<string, unknown>;
}

export interface RewriteTextResponse {
  text: string;
}

export interface SkillDraftRequest {
  instruction: string;
  existing_skills?: string[];
}

export interface SkillDraftResponse {
  skill: SkillDraft;
}

export interface JavaScriptDraftRequest {
  kind: "tool" | "hook";
  instruction: string;
  existing_ids?: string[];
  current_tool?: JavaScriptToolConfig;
  current_hook?: JavaScriptHookConfig;
}

export interface JavaScriptDraftResponse {
  kind: "tool" | "hook";
  tool?: JavaScriptToolConfig;
  hook?: JavaScriptHookConfig;
}

export function generateAgentDrafts(body: AgentDraftRequest, init?: RequestInit) {
  return post<AgentDraftResponse>("/api/fkteams/ai/agents/draft", body, init);
}

export function rewriteText(body: RewriteTextRequest) {
  return post<RewriteTextResponse>("/api/fkteams/ai/text/rewrite", body);
}

export function generateSkillDraft(body: SkillDraftRequest, init?: RequestInit) {
  return post<SkillDraftResponse>("/api/fkteams/ai/skills/draft", body, init);
}

export function generateJavaScriptDraft(body: JavaScriptDraftRequest, init?: RequestInit) {
  return post<JavaScriptDraftResponse>("/api/fkteams/ai/javascript/draft", body, init);
}
