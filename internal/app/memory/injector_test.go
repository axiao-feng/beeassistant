package memory

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestBuildMemoryContextCompactsLongEntries(t *testing.T) {
	longSummary := strings.Repeat("偏好", 100)
	longDetail := strings.Repeat("请使用简洁明确的中文回答。 ", 40)
	context := BuildMemoryContext([]MemoryEntry{{
		Type:    Preference,
		Summary: longSummary,
		Detail:  longDetail,
	}})

	if context == "" || !strings.Contains(context, "## 长期记忆") {
		t.Fatalf("context = %q, want memory heading", context)
	}
	if !strings.Contains(context, "…") {
		t.Fatalf("context = %q, want compacted ellipsis", context)
	}
	if utf8.RuneCountInString(context) > memoryContextContentBudget+utf8.RuneCountInString(memoryUsageGuide)+128 {
		t.Fatalf("context has too many runes: %d", utf8.RuneCountInString(context))
	}
}

func TestBuildMemoryContextNormalizesWhitespace(t *testing.T) {
	context := BuildMemoryContext([]MemoryEntry{{
		Type:    Feedback,
		Summary: "不要\n反复确认",
		Detail:  "能够安全推进时\t直接执行",
	}})

	if strings.Contains(context, "不要\n反复") || strings.Contains(context, "时\t直接") {
		t.Fatalf("context = %q, want normalized whitespace", context)
	}
	if !strings.Contains(context, "不要 反复确认") || !strings.Contains(context, "时 直接执行") {
		t.Fatalf("context = %q, want normalized text", context)
	}
}
