package javascript

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"

	"fkteams/internal/app/config"
	"fkteams/internal/domain/session"
	runtimelog "fkteams/internal/runtime/log"
)

const (
	PermissionStorage     = config.JavaScriptPermissionStorage
	PermissionEventNotice = config.JavaScriptPermissionEventNotice
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
	id          string
	permissions []string
}

type invocationView struct {
	name   string
	callID string
}

func (c *capabilityContext) value() map[string]any {
	sessionID, _ := session.IDFromContext(c.ctx)
	return map[string]any{
		"tool_name":   c.invocation.name,
		"call_id":     c.invocation.callID,
		"session_id":  sessionID,
		"permissions": append([]string(nil), c.definition.permissions...),
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
	if !c.allowsTool(group, name) {
		return nil, fmt.Errorf("permission denied: tools:%s/%s", group, name)
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

func (c *capabilityContext) getState(key string) (any, error) {
	if err := c.require(PermissionStorage, c.options.StateStore != nil); err != nil {
		return nil, err
	}
	value, exists, err := c.options.StateStore.Get(c.definition.id, key)
	if err != nil || !exists {
		return nil, err
	}
	return value, nil
}

func (c *capabilityContext) setState(key string, value any) error {
	if err := c.require(PermissionStorage, c.options.StateStore != nil); err != nil {
		return err
	}
	return c.options.StateStore.Set(c.definition.id, key, value)
}

func (c *capabilityContext) deleteState(key string) (bool, error) {
	if err := c.require(PermissionStorage, c.options.StateStore != nil); err != nil {
		return false, err
	}
	return c.options.StateStore.Delete(c.definition.id, key)
}

func (c *capabilityContext) stateKeys() ([]string, error) {
	if err := c.require(PermissionStorage, c.options.StateStore != nil); err != nil {
		return nil, err
	}
	return c.options.StateStore.Keys(c.definition.id)
}

func (c *capabilityContext) notice(message, level string) error {
	if err := c.require(PermissionEventNotice, c.options.NoticeEmitter != nil); err != nil {
		return err
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return fmt.Errorf("notice message is required")
	}
	if len(message) > 8<<10 {
		return fmt.Errorf("notice message exceeds 8192 bytes")
	}
	if level == "" {
		level = "info"
	}
	if level != "info" && level != "warn" && level != "error" {
		return fmt.Errorf("notice level must be info, warn or error")
	}
	return c.options.NoticeEmitter.Notice(c.ctx, c.definition.id, level, message)
}

func (c *capabilityContext) require(permission string, available bool) error {
	if !c.hasPermission(permission) {
		return fmt.Errorf("permission denied: %s", permission)
	}
	if !available {
		return fmt.Errorf("capability is unavailable: %s", permission)
	}
	return nil
}

func (c *capabilityContext) allowsTool(group, name string) bool {
	return c.hasPermission("tools:"+group) || c.hasPermission("tools:"+group+"/"+name)
}

func (c *capabilityContext) hasPermission(permission string) bool {
	for _, candidate := range c.definition.permissions {
		if candidate == permission {
			return true
		}
	}
	return false
}

func decodeToolContent(content string) any {
	var value any
	if json.Unmarshal([]byte(content), &value) == nil {
		return value
	}
	return content
}
