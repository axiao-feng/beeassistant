package javascript

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"fkteams/internal/app/config"
	runtimeport "fkteams/internal/ports/runtime"
)

type fakeToolCaller struct {
	request ToolCallRequest
	content string
}

func (f *fakeToolCaller) Call(_ context.Context, request ToolCallRequest) (string, error) {
	f.request = request
	if f.content != "" {
		return f.content, nil
	}
	return `{"content":"hello","value":42}`, nil
}

type memoryStateStore struct {
	values map[string]any
}

func (s *memoryStateStore) Get(namespace, key string) (any, bool, error) {
	value, ok := s.values[namespace+":"+key]
	return value, ok, nil
}

func (s *memoryStateStore) Set(namespace, key string, value any) error {
	s.values[namespace+":"+key] = value
	return nil
}

func (s *memoryStateStore) Delete(namespace, key string) (bool, error) {
	storageKey := namespace + ":" + key
	_, ok := s.values[storageKey]
	delete(s.values, storageKey)
	return ok, nil
}

func (s *memoryStateStore) Keys(string) ([]string, error) { return []string{"counter"}, nil }

type fakeNoticeEmitter struct {
	level   string
	message string
}

func (f *fakeNoticeEmitter) Notice(_ context.Context, _, level, message string) error {
	f.level = level
	f.message = message
	return nil
}

func TestToolInvokesJavaScriptWithArgumentsAndContext(t *testing.T) {
	tool, err := NewTool(config.JavaScriptTool{
		ID:          "greet",
		Name:        "问候",
		Description: "生成问候语",
		Source: `function execute(input, context) {
  return {message: "hello " + input.name, tool: context.tool_name};
}`,
	})
	if err != nil {
		t.Fatalf("NewTool() error = %v", err)
	}
	result, err := tool.Invoke(context.Background(), runtimeport.ToolInvocation{
		Name:      "greet",
		Arguments: `{"name":"Ada"}`,
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Content != `{"message":"hello Ada","tool":"greet"}` {
		t.Fatalf("Invoke() content = %s", result.Content)
	}
}

func TestNewToolRejectsMissingExecuteFunction(t *testing.T) {
	_, err := NewTool(config.JavaScriptTool{ID: "invalid", Source: `function other() {}`})
	if err == nil {
		t.Fatal("NewTool() error = nil, want missing execute error")
	}
}

func TestToolCapabilitiesComposeToolsAndPersistState(t *testing.T) {
	caller := &fakeToolCaller{}
	state := &memoryStateStore{values: make(map[string]any)}
	notices := &fakeNoticeEmitter{}
	tool, err := NewToolWithOptions(config.JavaScriptTool{
		ID:          "workflow",
		Name:        "工作流",
		Description: "组合已有能力",
		Source: `function execute(input, context) {
  const previous = context.state.get("counter") || 0;
  context.state.set("counter", previous + 1);
  const content = context.workspace.read(input.path, {start_line: 2});
  context.notify("finished", "warn");
  return {counter: context.state.get("counter"), content, keys: context.state.keys()};
}`,
	}, Options{ToolCaller: caller, StateStore: state, NoticeEmitter: notices})
	if err != nil {
		t.Fatalf("NewTool() error = %v", err)
	}
	result, err := tool.Invoke(context.Background(), runtimeport.ToolInvocation{
		Name:      "workflow",
		CallID:    "call-1",
		Arguments: `{"path":"README.md"}`,
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Content != `{"content":"hello","counter":1,"keys":["counter"]}` {
		t.Fatalf("Invoke() content = %s", result.Content)
	}
	if caller.request.Group != "file" || caller.request.Tool != "file_read" || caller.request.CallID != "call-1:js:1" {
		t.Fatalf("Call() request = %#v", caller.request)
	}
	if !reflect.DeepEqual(caller.request.Args, map[string]any{"filepath": "README.md", "start_line": int64(2)}) {
		t.Fatalf("Call() args = %#v", caller.request.Args)
	}
	if notices.level != "warn" || notices.message != "finished" {
		t.Fatalf("Notice() = %q/%q", notices.level, notices.message)
	}
}

func TestToolCapabilityRejectsRecursiveToolGroup(t *testing.T) {
	tool, err := NewToolWithOptions(config.JavaScriptTool{
		ID:          "recursive",
		Name:        "递归工具",
		Description: "验证递归保护",
		Source:      `function execute(input, context) { return context.tools.call("javascript", "recursive", {}); }`,
	}, Options{ToolCaller: &fakeToolCaller{}})
	if err != nil {
		t.Fatalf("NewTool() error = %v", err)
	}
	_, err = tool.Invoke(context.Background(), runtimeport.ToolInvocation{Name: "recursive"})
	if err == nil {
		t.Fatal("Invoke() error = nil, want recursive group error")
	}
}

func TestToolConvenienceFunctionPromotesToolError(t *testing.T) {
	tool, err := NewToolWithOptions(config.JavaScriptTool{
		ID:          "read_missing",
		Name:        "读取文件",
		Description: "验证宿主错误",
		Source:      `function execute(input, {workspace}) { return workspace.read(input.path); }`,
	}, Options{ToolCaller: &fakeToolCaller{content: `{"error_message":"file not found"}`}})
	if err != nil {
		t.Fatalf("NewTool() error = %v", err)
	}
	_, err = tool.Invoke(context.Background(), runtimeport.ToolInvocation{
		Name:      "read_missing",
		Arguments: `{"path":"missing.txt"}`,
	})
	if err == nil || !strings.Contains(err.Error(), "file not found") {
		t.Fatalf("Invoke() error = %v, want host tool error", err)
	}
}

func TestToolStoragePersistsAcrossIndependentRuntimes(t *testing.T) {
	store, err := NewFileStateStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStateStore() error = %v", err)
	}
	tool, err := NewToolWithOptions(config.JavaScriptTool{
		ID:          "persistent",
		Name:        "持久化工具",
		Description: "验证跨 Runtime 状态",
		Source: `function execute(input, context) {
  const state = context.state.get("state") || {count: 0};
  state.count += 1;
  context.state.set("state", state);
  return state;
}`,
	}, Options{StateStore: store})
	if err != nil {
		t.Fatalf("NewToolWithOptions() error = %v", err)
	}
	for count := 1; count <= 2; count++ {
		result, err := tool.Invoke(context.Background(), runtimeport.ToolInvocation{Name: "persistent"})
		if err != nil {
			t.Fatalf("Invoke(%d) error = %v", count, err)
		}
		want := fmt.Sprintf(`{"count":%d}`, count)
		if result.Content != want {
			t.Fatalf("Invoke(%d) content = %s, want %s", count, result.Content, want)
		}
	}
}
