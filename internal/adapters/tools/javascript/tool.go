// Package javascript 将用户 JavaScript 脚本适配为运行时工具。
package javascript

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	jsruntime "fkteams/internal/adapters/javascript"
	"fkteams/internal/app/config"
	runtimeport "fkteams/internal/ports/runtime"
)

// Tool 是由配置动态创建的 goja 工具。
type Tool struct {
	definition config.JavaScriptTool
	program    *jsruntime.Program
}

// NewTool 编译并创建 JavaScript 工具。
func NewTool(definition config.JavaScriptTool) (*Tool, error) {
	program, err := jsruntime.Compile("tool:"+definition.ID, definition.Source)
	if err != nil {
		return nil, err
	}
	if err := program.ValidateFunction(context.Background(), jsruntime.DefaultTimeout, "execute"); err != nil {
		return nil, err
	}
	return &Tool{definition: definition, program: program}, nil
}

// ValidateDefinitions 编译所有启用的工具脚本。
func ValidateDefinitions(definitions []config.JavaScriptTool) error {
	for _, definition := range definitions {
		if !definition.Enabled {
			continue
		}
		if _, err := NewTool(definition); err != nil {
			return fmt.Errorf("javascript tool %s: %w", definition.ID, err)
		}
	}
	return nil
}

// Info 返回提供给模型的工具名称、描述和安全策略。
func (t *Tool) Info(context.Context) (*runtimeport.ToolInfo, error) {
	if t == nil {
		return nil, fmt.Errorf("javascript tool is nil")
	}
	return &runtimeport.ToolInfo{
		Name: t.definition.ID,
		Desc: t.definition.Description,
		Policy: runtimeport.ToolPolicyMetadata{
			ReadOnly:    t.definition.ReadOnly,
			Destructive: !t.definition.ReadOnly,
			Serialize:   !t.definition.ReadOnly,
		},
	}, nil
}

// InputSchema 返回用户配置的 JSON Schema 参数定义。
func (t *Tool) InputSchema() map[string]any {
	if t == nil {
		return nil
	}
	return t.definition.Parameters
}

// Invoke 执行全局 execute(input, context) 函数并序列化返回值。
func (t *Tool) Invoke(ctx context.Context, invocation runtimeport.ToolInvocation) (*runtimeport.ToolResult, error) {
	if t == nil || t.program == nil {
		return nil, fmt.Errorf("javascript tool is not initialized")
	}
	input := make(map[string]any)
	if invocation.Arguments != "" {
		if err := json.Unmarshal([]byte(invocation.Arguments), &input); err != nil {
			return nil, fmt.Errorf("decode javascript tool arguments: %w", err)
		}
	}
	timeout := time.Duration(t.definition.TimeoutMS) * time.Millisecond
	result, err := t.program.Call(ctx, timeout, "execute", input, map[string]any{
		"tool_name": invocation.Name,
		"call_id":   invocation.CallID,
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return &runtimeport.ToolResult{}, nil
	}
	if text, ok := result.(string); ok {
		return &runtimeport.ToolResult{Content: text}, nil
	}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("encode javascript tool result: %w", err)
	}
	return &runtimeport.ToolResult{Content: string(data)}, nil
}
