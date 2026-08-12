package javascript

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"

	"fkteams/internal/domain/session"
	runtimelog "fkteams/internal/runtime/log"
)

// ToolCallRequest 描述脚本对已有工具的一次受控调用。
type ToolCallRequest struct {
	ScriptID string
	Group    string
	Tool     string
	CallID   string
	Args     map[string]any
}

// ToolCaller 把脚本工具调用路由到应用工具注册表。
type ToolCaller interface {
	Call(ctx context.Context, request ToolCallRequest) (string, error)
}

// StateStore 提供按脚本 ID 隔离的 JSON 状态存储。
type StateStore interface {
	Get(namespace, key string) (any, bool, error)
	Set(namespace, key string, value any) error
	Delete(namespace, key string) (bool, error)
	Keys(namespace string) ([]string, error)
}

// NoticeEmitter 向当前任务事件流发送脚本通知。
type NoticeEmitter interface {
	Notice(ctx context.Context, scriptID, level, message string) error
}

// Options 配置 JavaScript 工具可使用的宿主能力。
type Options struct {
	ToolCaller    ToolCaller
	StateStore    StateStore
	NoticeEmitter NoticeEmitter
}

type capabilityContext struct {
	ctx        context.Context
	definition configView
	invocation invocationView
	options    Options
	calls      atomic.Uint64
}

type configView struct {
	id string
}

type invocationView struct {
	name   string
	callID string
}

func (c *capabilityContext) value() map[string]any {
	sessionID, _ := session.IDFromContext(c.ctx)
	return map[string]any{
		"tool_name":  c.invocation.name,
		"call_id":    c.invocation.callID,
		"session_id": sessionID,
		"workspace": map[string]any{
			"read":    c.readFile,
			"write":   c.writeFile,
			"append":  c.appendFile,
			"replace": c.replaceFile,
			"patch":   c.patchFiles,
			"list":    c.listFiles,
			"glob":    c.globFiles,
			"grep":    c.grepFiles,
		},
		"shell": map[string]any{
			"run": c.runCommand,
		},
		"web": map[string]any{
			"search": c.searchWeb,
			"fetch":  c.fetchWeb,
		},
		"state": map[string]any{
			"get":    c.getState,
			"set":    c.setState,
			"delete": c.deleteState,
			"keys":   c.stateKeys,
		},
		"notify": c.notice,
		// 保留旧名称，避免现有用户脚本在升级后失效。
		"tools": map[string]any{
			"call": c.callTool,
		},
		"storage": map[string]any{
			"get":    c.getState,
			"set":    c.setState,
			"delete": c.deleteState,
			"keys":   c.stateKeys,
		},
		"events": map[string]any{
			"notice": c.notice,
		},
		"log": map[string]any{
			"debug": func(value any) { runtimelog.Debugf("[javascript:%s] %v", c.definition.id, value) },
			"info":  func(value any) { runtimelog.Infof("[javascript:%s] %v", c.definition.id, value) },
			"warn":  func(value any) { runtimelog.Warnf("[javascript:%s] %v", c.definition.id, value) },
			"error": func(value any) { runtimelog.Errorf("[javascript:%s] %v", c.definition.id, value) },
		},
	}
}

func (c *capabilityContext) callTool(group, name string, args map[string]any) (any, error) {
	group = strings.TrimSpace(group)
	name = strings.TrimSpace(name)
	if group == "" || name == "" {
		return nil, fmt.Errorf("tool group and name are required")
	}
	if group == "javascript" {
		return nil, fmt.Errorf("javascript tools cannot call the javascript tool group")
	}
	if c.options.ToolCaller == nil {
		return nil, fmt.Errorf("tool calling capability is unavailable")
	}
	sequence := c.calls.Add(1)
	callID := fmt.Sprintf("%s:js:%d", c.invocation.callID, sequence)
	content, err := c.options.ToolCaller.Call(c.ctx, ToolCallRequest{
		ScriptID: c.definition.id,
		Group:    group,
		Tool:     name,
		CallID:   callID,
		Args:     args,
	})
	if err != nil {
		return nil, err
	}
	return decodeToolContent(content), nil
}

func (c *capabilityContext) readFile(path string, options ...map[string]any) (any, error) {
	return c.resultField("file", "file_read", hostArgs(options, map[string]any{"filepath": path}), "content")
}

func (c *capabilityContext) writeFile(path, content string) (any, error) {
	return c.callHostTool("file", "file_write", map[string]any{"filepath": path, "content": content})
}

func (c *capabilityContext) appendFile(path, content string) (any, error) {
	return c.callHostTool("file", "file_append", map[string]any{"filepath": path, "content": content})
}

func (c *capabilityContext) replaceFile(path, oldText, newText string) (any, error) {
	return c.callHostTool("file", "file_edit", map[string]any{
		"filepath":   path,
		"old_string": oldText,
		"new_string": newText,
	})
}

func (c *capabilityContext) patchFiles(patch string) (any, error) {
	return c.callHostTool("file", "file_patch", map[string]any{"patch": patch})
}

func (c *capabilityContext) listFiles(path string) (any, error) {
	return c.resultField("file", "file_list", map[string]any{"dirpath": path}, "content")
}

func (c *capabilityContext) globFiles(pattern string, options ...map[string]any) (any, error) {
	return c.resultField("file", "glob", hostArgs(options, map[string]any{"pattern": pattern}), "files")
}

func (c *capabilityContext) grepFiles(pattern string, options ...map[string]any) (any, error) {
	return c.resultField("file", "grep", hostArgs(options, map[string]any{"pattern": pattern}), "matches")
}

func (c *capabilityContext) runCommand(command string, options ...map[string]any) (any, error) {
	args := hostArgs(options, map[string]any{"command": command})
	if reason, _ := args["reason"].(string); strings.TrimSpace(reason) == "" {
		args["reason"] = "run by custom tool " + c.definition.id
	}
	return c.callHostTool("command", "execute", args)
}

func (c *capabilityContext) searchWeb(query string, options ...map[string]any) (any, error) {
	return c.resultField("search", "search", hostArgs(options, map[string]any{"query": query}), "results")
}

func (c *capabilityContext) fetchWeb(url string, options ...map[string]any) (any, error) {
	return c.resultField("fetch", "fetch", hostArgs(options, map[string]any{"url": url}), "content")
}

func (c *capabilityContext) callHostTool(group, name string, args map[string]any) (any, error) {
	result, err := c.callTool(group, name, args)
	if err != nil {
		return nil, err
	}
	if message := resultErrorMessage(result); message != "" {
		return nil, fmt.Errorf("%s", message)
	}
	return result, nil
}

func (c *capabilityContext) resultField(group, name string, args map[string]any, field string) (any, error) {
	result, err := c.callHostTool(group, name, args)
	if err != nil {
		return nil, err
	}
	object, ok := result.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s returned an invalid response", name)
	}
	return object[field], nil
}

func hostArgs(options []map[string]any, required map[string]any) map[string]any {
	args := make(map[string]any)
	if len(options) > 0 {
		for key, value := range options[0] {
			args[key] = value
		}
	}
	for key, value := range required {
		args[key] = value
	}
	return args
}

func resultErrorMessage(result any) string {
	object, ok := result.(map[string]any)
	if !ok {
		return ""
	}
	message, _ := object["error_message"].(string)
	return strings.TrimSpace(message)
}

func (c *capabilityContext) getState(key string) (any, error) {
	if c.options.StateStore == nil {
		return nil, fmt.Errorf("state capability is unavailable")
	}
	value, exists, err := c.options.StateStore.Get(c.definition.id, key)
	if err != nil || !exists {
		return nil, err
	}
	return value, nil
}

func (c *capabilityContext) setState(key string, value any) error {
	if c.options.StateStore == nil {
		return fmt.Errorf("state capability is unavailable")
	}
	return c.options.StateStore.Set(c.definition.id, key, value)
}

func (c *capabilityContext) deleteState(key string) (bool, error) {
	if c.options.StateStore == nil {
		return false, fmt.Errorf("state capability is unavailable")
	}
	return c.options.StateStore.Delete(c.definition.id, key)
}

func (c *capabilityContext) stateKeys() ([]string, error) {
	if c.options.StateStore == nil {
		return nil, fmt.Errorf("state capability is unavailable")
	}
	return c.options.StateStore.Keys(c.definition.id)
}

func (c *capabilityContext) notice(message string, levels ...string) error {
	if c.options.NoticeEmitter == nil {
		return fmt.Errorf("notification capability is unavailable")
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return fmt.Errorf("notice message is required")
	}
	if len(message) > 8<<10 {
		return fmt.Errorf("notice message exceeds 8192 bytes")
	}
	level := "info"
	if len(levels) > 0 && levels[0] != "" {
		level = levels[0]
	}
	if level != "info" && level != "warn" && level != "error" {
		return fmt.Errorf("notice level must be info, warn or error")
	}
	return c.options.NoticeEmitter.Notice(c.ctx, c.definition.id, level, message)
}

func decodeToolContent(content string) any {
	var value any
	if json.Unmarshal([]byte(content), &value) == nil {
		return value
	}
	return content
}
