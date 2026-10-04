package handler

import "testing"

func TestSessionSearchSnippet(t *testing.T) {
	snippet, ok := sessionSearchSnippet("今天讨论了蜜蜂助手的长期记忆设计。", "长期记忆")
	if !ok || snippet == "" {
		t.Fatalf("snippet = %q, ok = %v", snippet, ok)
	}
	if _, ok := sessionSearchSnippet("没有相关内容", "不存在"); ok {
		t.Fatal("unmatched query should not produce a result")
	}
}
