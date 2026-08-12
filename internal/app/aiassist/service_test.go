package aiassist

import (
	"context"
	"strings"
	"testing"

	domainmessage "fkteams/internal/domain/message"
	"fkteams/internal/testmodel"
)

func TestGenerateAgentsNormalizesDrafts(t *testing.T) {
	model := testmodel.New(domainmessage.Message{
		Role: domainmessage.RoleAssistant,
		Content: `{
  "agents": [
    {
      "id": "Research Agent",
      "name": "资料研究员",
      "description": "检索并整理资料",
      "prompt": "你是资料研究员。",
      "tools": ["fetch", "unknown", "fetch"],
      "enabled": false
    }
  ]
}`,
	})
	service := New(model)

	got, err := service.GenerateAgents(context.Background(), AgentDraftRequest{
		Instruction:     "创建研究智能体",
		ExistingAgents:  []string{"research_agent"},
		AvailableTools:  []string{"fetch"},
		AvailableModels: []string{"default-model"},
	})
	if err != nil {
		t.Fatalf("GenerateAgents returned error: %v", err)
	}
	if len(got.Agents) != 1 {
		t.Fatalf("agent count = %d, want 1", len(got.Agents))
	}
	agent := got.Agents[0]
	if agent.ID != "research_agent_2" {
		t.Fatalf("id = %q, want research_agent_2", agent.ID)
	}
	if agent.ModelID != "default-model" {
		t.Fatalf("model id = %q, want default-model", agent.ModelID)
	}
	if len(agent.Tools) != 1 || agent.Tools[0] != "fetch" {
		t.Fatalf("tools = %#v, want only fetch", agent.Tools)
	}
	if !agent.Enabled {
		t.Fatal("agent should be enabled")
	}
}

func TestRewriteTextDecodesFencedJSON(t *testing.T) {
	model := testmodel.New(domainmessage.Message{
		Role:    domainmessage.RoleAssistant,
		Content: "```json\n{\"text\":\"改写后的提示词\"}\n```",
	})
	service := New(model)

	got, err := service.RewriteText(context.Background(), RewriteTextRequest{
		Scenario:    "agent_prompt",
		Instruction: "更简洁",
		Text:        "旧提示词",
	})
	if err != nil {
		t.Fatalf("RewriteText returned error: %v", err)
	}
	if got.Text != "改写后的提示词" {
		t.Fatalf("text = %q, want rewritten text", got.Text)
	}
}

func TestGenerateSkillNormalizesDraft(t *testing.T) {
	model := testmodel.New(domainmessage.Message{
		Role: domainmessage.RoleAssistant,
		Content: `{
  "skill": {
    "slug": "Research Skill",
    "name": "资料整理技能",
    "description": "整理研究资料",
    "content": "---\nname: 资料整理技能\ndescription: 整理研究资料\n---\n\n# 资料整理技能\n"
  }
}`,
	})
	service := New(model)

	got, err := service.GenerateSkill(context.Background(), SkillDraftRequest{
		Instruction:    "创建资料整理技能",
		ExistingSkills: []string{"research_skill"},
	})
	if err != nil {
		t.Fatalf("GenerateSkill returned error: %v", err)
	}
	if got.Skill.Slug != "research_skill_2" {
		t.Fatalf("slug = %q, want research_skill_2", got.Skill.Slug)
	}
	if got.Skill.Name != "资料整理技能" || got.Skill.Description != "整理研究资料" {
		t.Fatalf("skill metadata = %#v", got.Skill)
	}
	if !strings.Contains(got.Skill.Content, "SKILL") && !strings.Contains(got.Skill.Content, "资料整理技能") {
		t.Fatalf("content = %q, want skill content", got.Skill.Content)
	}
}

func TestGenerateAgentsSendsInstructionToModel(t *testing.T) {
	model := testmodel.New(domainmessage.Message{Role: domainmessage.RoleAssistant, Content: `{"agents":[{"name":"A"}]}`})
	service := New(model)

	_, err := service.GenerateAgents(context.Background(), AgentDraftRequest{Instruction: "创建 A"})
	if err != nil {
		t.Fatalf("GenerateAgents returned error: %v", err)
	}
	calls := model.GenerateCalls()
	if len(calls) != 1 {
		t.Fatalf("generate calls = %d, want 1", len(calls))
	}
	if len(calls[0].Input) != 2 {
		t.Fatalf("input count = %d, want 2", len(calls[0].Input))
	}
	if !strings.Contains(calls[0].Input[1].Content, "创建 A") {
		t.Fatalf("input payload = %q, want instruction", calls[0].Input[1].Content)
	}
}

func TestGenerateJavaScriptToolNormalizesDraft(t *testing.T) {
	model := testmodel.New(domainmessage.Message{
		Role: domainmessage.RoleAssistant,
		Content: `{
  "kind": "tool",
  "tool": {
    "id": "Text Stats",
    "name": "文本统计",
    "description": "统计文本",
    "timeout_ms": 0,
    "read_only": true,
    "parameters": {"type":"object","properties":{"text":{"type":"string"}}},
    "source": "function execute(input) { return {length: input.text.length}; }"
  }
}`,
	})
	service := New(model)

	got, err := service.GenerateJavaScript(context.Background(), JavaScriptDraftRequest{
		Kind:        "tool",
		Instruction: "生成文本统计工具",
		ExistingIDs: []string{"text_stats"},
	})
	if err != nil {
		t.Fatalf("GenerateJavaScript() error = %v", err)
	}
	if got.Tool == nil || got.Tool.ID != "text_stats_2" {
		t.Fatalf("tool = %#v", got.Tool)
	}
	if got.Tool.Enabled || got.Tool.TimeoutMS != 200 {
		t.Fatalf("tool defaults = %#v", got.Tool)
	}
}

func TestGenerateJavaScriptHookFiltersHookPoints(t *testing.T) {
	model := testmodel.New(domainmessage.Message{
		Role: domainmessage.RoleAssistant,
		Content: `{
  "kind": "hook",
  "hook": {
    "id": "guard",
    "name": "保护规则",
    "hook_points": ["before_tool_call", "unknown"],
    "source": "function handle(hook) { return {action: 'continue'}; }"
  }
}`,
	})
	service := New(model)

	got, err := service.GenerateJavaScript(context.Background(), JavaScriptDraftRequest{
		Kind:        "hook",
		Instruction: "生成保护规则",
	})
	if err != nil {
		t.Fatalf("GenerateJavaScript() error = %v", err)
	}
	if got.Hook == nil || len(got.Hook.HookPoints) != 1 || got.Hook.HookPoints[0] != "before_tool_call" {
		t.Fatalf("hook = %#v", got.Hook)
	}
	if got.Hook.ErrorPolicy != "fail" {
		t.Fatalf("error policy = %q", got.Hook.ErrorPolicy)
	}
}
