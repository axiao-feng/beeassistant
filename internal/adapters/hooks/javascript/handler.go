// Package javascript 将用户 JavaScript 脚本适配为运行时 hooks。
package javascript

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	jsruntime "fkteams/internal/adapters/javascript"
	"fkteams/internal/app/config"
	"fkteams/internal/domain/event"
	"fkteams/internal/domain/message"
	hookport "fkteams/internal/ports/hooks"
	"fkteams/internal/runtime/log"
)

var allHookPoints = []hookport.HookPoint{
	hookport.HookBeforeRun,
	hookport.HookAfterRun,
	hookport.HookOnEvent,
	hookport.HookBeforeToolCall,
	hookport.HookAfterToolCall,
	hookport.HookBeforeModelRequest,
	hookport.HookAfterModelResponse,
}

// Source 返回最新的 JavaScript hook 配置快照。
type Source func() []config.JavaScriptHook

type cachedProgram struct {
	source  string
	program *jsruntime.Program
}

// Handler 按配置顺序执行匹配当前扩展点的脚本。
type Handler struct {
	source Source
	mu     sync.RWMutex
	cache  map[string]cachedProgram
}

// NewHandler 创建能够感知配置热更新的 JavaScript hook 处理器。
func NewHandler(source Source) *Handler {
	return &Handler{source: source, cache: make(map[string]cachedProgram)}
}

func (h *Handler) Name() string { return "javascript-hooks" }

func (h *Handler) Points() []hookport.HookPoint {
	return append([]hookport.HookPoint(nil), allHookPoints...)
}

// Handle 依次执行匹配的脚本，并把前一个脚本的载荷传给下一个脚本。
func (h *Handler) Handle(ctx context.Context, inv hookport.Invocation) (hookport.Result, error) {
	payload := inv.Payload
	result := hookport.Result{Payload: payload, Action: hookport.ActionContinue}
	for _, definition := range h.definitions(inv.HookPoint) {
		inv.Payload = payload
		next, err := h.invoke(ctx, definition, inv)
		if err != nil {
			if h.continueAfterError(definition, inv.HookPoint, err) {
				continue
			}
			return result, err
		}
		if next.Payload != nil {
			payload = next.Payload
			result.Payload = payload
		}
		if next.Action != "" {
			result.Action = next.Action
		}
		if next.Message != "" {
			result.Message = next.Message
		}
		if result.Action == hookport.ActionSkip || result.Action == hookport.ActionReject {
			return result, nil
		}
	}
	return result, nil
}

func (h *Handler) definitions(point hookport.HookPoint) []config.JavaScriptHook {
	if h == nil || h.source == nil {
		return nil
	}
	source := h.source()
	definitions := make([]config.JavaScriptHook, 0, len(source))
	for _, item := range source {
		if item.Enabled && containsPoint(item.HookPoints, point) {
			definitions = append(definitions, item)
		}
	}
	sort.SliceStable(definitions, func(i, j int) bool {
		return definitions[i].Priority < definitions[j].Priority
	})
	return definitions
}

func containsPoint(points []string, point hookport.HookPoint) bool {
	for _, item := range points {
		if hookport.HookPoint(item) == point {
			return true
		}
	}
	return false
}

func (h *Handler) invoke(ctx context.Context, definition config.JavaScriptHook, inv hookport.Invocation) (hookport.Result, error) {
	program, err := h.program(definition)
	if err != nil {
		return hookport.Result{}, err
	}
	payload, err := payloadForScript(inv.Payload)
	if err != nil {
		return hookport.Result{}, err
	}
	payload, err = mutableJSONValue(payload)
	if err != nil {
		return hookport.Result{}, err
	}
	timeout := time.Duration(definition.TimeoutMS) * time.Millisecond
	value, err := program.Call(ctx, timeout, "handle", map[string]any{
		"point":      inv.HookPoint,
		"session_id": inv.SessionID,
		"run_id":     inv.RunID,
		"turn_id":    inv.TurnID,
		"payload":    payload,
	})
	if err != nil {
		return hookport.Result{}, fmt.Errorf("javascript hook %s: %w", definition.ID, err)
	}
	return resultFromScript(value, inv.Payload)
}

func mutableJSONValue(value any) (any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode javascript hook payload: %w", err)
	}
	var result any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode javascript hook payload: %w", err)
	}
	return result, nil
}

func (h *Handler) program(definition config.JavaScriptHook) (*jsruntime.Program, error) {
	h.mu.RLock()
	cached, ok := h.cache[definition.ID]
	h.mu.RUnlock()
	if ok && cached.source == definition.Source {
		return cached.program, nil
	}
	program, err := compileDefinition(definition)
	if err != nil {
		return nil, err
	}
	h.mu.Lock()
	h.cache[definition.ID] = cachedProgram{source: definition.Source, program: program}
	h.mu.Unlock()
	return program, nil
}

func compileDefinition(definition config.JavaScriptHook) (*jsruntime.Program, error) {
	program, err := jsruntime.Compile("hook:"+definition.ID, definition.Source)
	if err != nil {
		return nil, fmt.Errorf("javascript hook %s: %w", definition.ID, err)
	}
	if err := program.ValidateFunction(context.Background(), jsruntime.DefaultTimeout, "handle"); err != nil {
		return nil, fmt.Errorf("javascript hook %s: %w", definition.ID, err)
	}
	return program, nil
}

// ValidateDefinitions 编译所有启用的 hook 脚本。
func ValidateDefinitions(definitions []config.JavaScriptHook) error {
	for _, definition := range definitions {
		if !definition.Enabled {
			continue
		}
		if _, err := compileDefinition(definition); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) continueAfterError(definition config.JavaScriptHook, point hookport.HookPoint, err error) bool {
	policy := hookport.ErrorPolicy(definition.ErrorPolicy)
	if policy == "" {
		policy = defaultErrorPolicy(point)
	}
	switch policy {
	case hookport.ErrorIgnore:
		return true
	case hookport.ErrorWarn:
		log.Printf("javascript hook warning: point=%s id=%s err=%v", point, definition.ID, err)
		return true
	default:
		return false
	}
}

func defaultErrorPolicy(point hookport.HookPoint) hookport.ErrorPolicy {
	switch point {
	case hookport.HookOnEvent, hookport.HookAfterRun, hookport.HookAfterToolCall, hookport.HookAfterModelResponse:
		return hookport.ErrorWarn
	default:
		return hookport.ErrorFail
	}
}

type turnInputDTO struct {
	Context []message.Message `json:"context"`
	Message message.Message   `json:"message"`
}

func payloadForScript(payload hookport.Payload) (any, error) {
	switch typed := payload.(type) {
	case hookport.BeforeRunPayload:
		return map[string]any{"input": turnInputDTO{Context: typed.Input.Context, Message: typed.Input.Message}}, nil
	case hookport.AfterRunPayload:
		result := any(nil)
		if typed.Result != nil {
			result = map[string]any{"last_event": typed.Result.LastEvent}
		}
		return map[string]any{
			"input":  turnInputDTO{Context: typed.Input.Context, Message: typed.Input.Message},
			"result": result,
			"error":  errorText(typed.Error),
		}, nil
	case hookport.EventPayload:
		return map[string]any{"event": typed.Event}, nil
	case hookport.BeforeToolCallPayload:
		return map[string]any{"tool_name": typed.ToolName, "args": typed.Args, "meta": typed.Meta}, nil
	case hookport.AfterToolCallPayload:
		return map[string]any{
			"tool_name": typed.ToolName,
			"args":      typed.Args,
			"result":    typed.Result,
			"error":     errorText(typed.Error),
			"meta":      typed.Meta,
		}, nil
	case hookport.BeforeModelRequestPayload:
		return map[string]any{"messages": typed.Messages, "meta": typed.Meta}, nil
	case hookport.AfterModelResponsePayload:
		return map[string]any{
			"message": typed.Message,
			"usage":   typed.Usage,
			"error":   errorText(typed.Error),
			"meta":    typed.Meta,
		}, nil
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported javascript hook payload %T", payload)
	}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

type scriptResult struct {
	Action  hookport.Action `json:"action"`
	Message string          `json:"message"`
	Payload json.RawMessage `json:"payload"`
}

func resultFromScript(value any, original hookport.Payload) (hookport.Result, error) {
	result := hookport.Result{Payload: original, Action: hookport.ActionContinue}
	if value == nil {
		return result, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return result, fmt.Errorf("encode javascript hook result: %w", err)
	}
	var script scriptResult
	if err := json.Unmarshal(data, &script); err != nil {
		return result, fmt.Errorf("decode javascript hook result: %w", err)
	}
	switch script.Action {
	case "", hookport.ActionContinue:
		result.Action = hookport.ActionContinue
	case hookport.ActionSkip, hookport.ActionReject:
		result.Action = script.Action
	default:
		return result, fmt.Errorf("invalid javascript hook action: %s", script.Action)
	}
	result.Message = script.Message
	if len(script.Payload) == 0 || string(script.Payload) == "null" {
		return result, nil
	}
	result.Payload, err = decodePayload(script.Payload, original)
	if err != nil {
		return result, err
	}
	return result, nil
}

func decodePayload(data []byte, original hookport.Payload) (hookport.Payload, error) {
	switch typed := original.(type) {
	case hookport.BeforeRunPayload:
		var payload struct {
			Input turnInputDTO `json:"input"`
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			return original, err
		}
		typed.Input = message.TurnInput{Context: payload.Input.Context, Message: payload.Input.Message}
		return typed, nil
	case hookport.EventPayload:
		var payload struct {
			Event event.Event `json:"event"`
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			return original, err
		}
		typed.Event = payload.Event
		return typed, nil
	case hookport.BeforeToolCallPayload:
		var payload struct {
			Args string         `json:"args"`
			Meta map[string]any `json:"meta"`
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			return original, err
		}
		typed.Args = payload.Args
		typed.Meta = payload.Meta
		return typed, nil
	case hookport.BeforeModelRequestPayload:
		var payload struct {
			Messages []message.Message `json:"messages"`
			Meta     map[string]any    `json:"meta"`
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			return original, err
		}
		typed.Messages = payload.Messages
		typed.Meta = payload.Meta
		return typed, nil
	default:
		// after_* 载荷只用于观察，避免脚本伪造已经发生的执行结果。
		return original, nil
	}
}
