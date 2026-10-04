package memory

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestExtractParsesFencedJSONAndFiltersInvalidTypes(t *testing.T) {
	llm := &fakeLLMClient{response: "```json\n" + `[
		{"type":"preference","summary":"偏好简洁","detail":"希望回复直接明确","tags":["风格","简洁"]},
		{"type":"invalid","summary":"忽略","detail":"非法类型","tags":["bad"]}
	]` + "\n```"}

	entries, err := Extract(context.Background(), longConversationMessages(), "session-1", llm)
	if err != nil {
		t.Fatalf("Extract error: %v", err)
	}
	if llm.calls != 1 {
		t.Fatalf("llm calls = %d, want 1", llm.calls)
	}
	if !strings.Contains(llm.prompt, "[用户]") || !strings.Contains(llm.prompt, "[AI助手]") {
		t.Fatalf("prompt should include formatted conversation, got %q", llm.prompt)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %#v, want one valid entry", entries)
	}
	entry := entries[0]
	if entry.Type != Preference || entry.Summary != "偏好简洁" || entry.SessionID != "session-1" {
		t.Fatalf("entry = %#v", entry)
	}
	if entry.ID == "" || entry.CreatedAt.IsZero() {
		t.Fatalf("entry should have ID and CreatedAt: %#v", entry)
	}
}

func TestExtractSkipsShortConversationWithoutLLMCall(t *testing.T) {
	llm := &fakeLLMClient{response: "not json"}

	entries, err := Extract(context.Background(), []Message{{Role: "user", Content: "太短"}}, "session-1", llm)
	if err != nil {
		t.Fatalf("Extract error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("entries = %#v, want empty", entries)
	}
	if llm.calls != 0 {
		t.Fatalf("llm calls = %d, want 0", llm.calls)
	}
}

func TestExtractCompressesOversizedMemoryOnlyWhenNeeded(t *testing.T) {
	longSummary := strings.Repeat("用户偏好简洁回答", 20)
	longDetail := strings.Repeat("回答时先给结论，再补充必要说明。", 20)
	llm := &sequenceLLM{responses: []string{
		`[{"type":"preference","summary":"` + longSummary + `","detail":"` + longDetail + `","tags":["风格"]}]`,
		`{"summary":"偏好简洁","detail":"先给结论，再补充必要说明"}`,
	}}

	entries, err := Extract(context.Background(), longConversationMessages(), "session-1", llm)
	if err != nil {
		t.Fatalf("Extract error: %v", err)
	}
	if len(entries) != 1 || entries[0].Summary != "偏好简洁" || entries[0].Detail != "先给结论，再补充必要说明" {
		t.Fatalf("entries = %#v, want compressed memory", entries)
	}
	if llm.calls != 2 || !strings.Contains(llm.prompts[1], "压缩") {
		t.Fatalf("calls/prompts = %d/%#v, want one extraction and one compression", llm.calls, llm.prompts)
	}
}

func TestExtractReturnsLLMAndParseErrors(t *testing.T) {
	if _, err := Extract(context.Background(), longConversationMessages(), "session-1", &fakeLLMClient{err: errors.New("down")}); err == nil || !strings.Contains(err.Error(), "llm complete failed") {
		t.Fatalf("llm error = %v", err)
	}
	if _, err := Extract(context.Background(), longConversationMessages(), "session-1", &fakeLLMClient{response: "bad json"}); err == nil || !strings.Contains(err.Error(), "failed to parse llm response") {
		t.Fatalf("parse error = %v", err)
	}
}

func TestFormatConversationFiltersRolesAndTruncatesAssistant(t *testing.T) {
	text := formatConversation([]Message{
		{Role: "system", Content: strings.Repeat("忽略", 200)},
		{Role: "user", Content: strings.Repeat("用户", 120)},
		{Role: "assistant", Content: strings.Repeat("助手", 600)},
	})

	if strings.Contains(text, "system") || strings.Contains(text, "忽略") {
		t.Fatalf("conversation should filter non-chat roles: %q", text)
	}
	if !strings.Contains(text, "[用户]") || !strings.Contains(text, "[AI助手]") {
		t.Fatalf("conversation labels missing: %q", text)
	}
	if strings.Count(text, "助手") > 500 {
		t.Fatalf("assistant content should be truncated, got %d runes", strings.Count(text, "助手"))
	}
}

func longConversationMessages() []Message {
	return []Message{
		{Role: "user", Content: strings.Repeat("我喜欢简洁明确的中文回复。", 20)},
		{Role: "assistant", Content: strings.Repeat("收到，我会保持简洁。", 20)},
	}
}

type fakeLLMClient struct {
	response string
	err      error
	calls    int
	prompt   string
}

func (f *fakeLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	f.calls++
	f.prompt = prompt
	return f.response, f.err
}

type sequenceLLM struct {
	responses []string
	calls     int
	prompts   []string
}

func (s *sequenceLLM) Complete(_ context.Context, prompt string) (string, error) {
	s.prompts = append(s.prompts, prompt)
	response := s.responses[len(s.responses)-1]
	if s.calls < len(s.responses) {
		response = s.responses[s.calls]
	}
	s.calls++
	return response, nil
}
