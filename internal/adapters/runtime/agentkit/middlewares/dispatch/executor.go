package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fkteams/internal/adapters/model/providers/copilot"
	kitruntime "fkteams/internal/adapters/runtime/agentkit"
	"fkteams/internal/runtime/approval"
	"fkteams/internal/runtime/events"
	retry "fkteams/internal/runtime/retry"
	"fmt"
	"sync"

	kit "github.com/wsshow/agentkit"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

const (
	statusSuccess = "success"
	statusError   = "error"
	statusTimeout = "timeout"
)

// --- 输入输出 ---

type dispatchInput struct {
	Tasks []taskItem `json:"tasks" jsonschema:"description=子任务列表"`
}

type taskItem struct {
	Description string `json:"description" jsonschema:"description=子任务的详细描述，要足够清晰以便独立执行"`
}

type taskResult struct {
	TaskIndex   int      `json:"task_index"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Result      string   `json:"result,omitempty"`
	Error       string   `json:"error,omitempty"`
	Operations  []string `json:"operations,omitempty"`
}

// --- 并行执行 ---

func (m *middleware) executeTasks(ctx context.Context, input *dispatchInput) (string, error) {
	if len(input.Tasks) == 0 {
		return `{"results":[]}`, nil
	}

	details := make([]approval.OperationDetail, 0, len(input.Tasks))
	for i, t := range input.Tasks {
		details = append(details, approval.OperationDetail{
			Name:  fmt.Sprintf("Task %d", i+1),
			Value: t.Description,
		})
	}
	if err := approval.RequireOperation(ctx, approval.Operation{
		StoreName: approval.StoreDispatch,
		Key:       "dispatch_tasks",
		Title:     "Dispatch tasks require approval",
		Target:    fmt.Sprintf("%d autonomous tasks", len(input.Tasks)),
		Details:   details,
	}); err != nil {
		if errors.Is(err, approval.ErrRejected) {
			return `{"error":"用户取消分发"}`, nil
		}
		return "", err
	}

	eventCh := make(chan viewEvent, 64)
	results := make([]taskResult, len(input.Tasks))
	sem := semaphore.NewWeighted(m.maxConcurrency)

	// 可取消的上下文，用于 Ctrl+C 中断
	taskCtx, cancelTasks := context.WithCancel(ctx)
	defer cancelTasks()

	g, gCtx := errgroup.WithContext(taskCtx)
	var mu sync.Mutex

	for i, task := range input.Tasks {
		g.Go(func() error {
			if err := sem.Acquire(gCtx, 1); err != nil {
				mu.Lock()
				results[i] = taskResult{TaskIndex: i, Description: task.Description, Status: statusError, Error: "cancelled"}
				mu.Unlock()
				return nil
			}
			defer sem.Release(1)

			r := m.executeOneTask(taskCtx, i, task, eventCh)
			mu.Lock()
			results[i] = r
			mu.Unlock()
			return nil
		})
	}

	// 任务全部完成后关闭 channel
	go func() {
		g.Wait()
		close(eventCh)
	}()

	// 非交互模式（如 Web 服务）：跳过 Bubble Tea，将事件通过 fkevent 转发
	if events.IsNonInteractive(ctx) {
		forwardEvents(ctx, input.Tasks, eventCh)
	} else if cancelled := runDispatchView(input.Tasks, eventCh); cancelled {
		cancelTasks()
	}

	// 确保所有任务完成
	_ = g.Wait()

	data, err := json.Marshal(struct {
		Results []taskResult `json:"results"`
	}{results})
	if err != nil {
		return "", fmt.Errorf("marshal results: %w", err)
	}
	return string(data), nil
}

func (m *middleware) executeOneTask(parentCtx context.Context, index int, task taskItem, ch chan<- viewEvent) taskResult {
	result := taskResult{TaskIndex: index, Description: task.Description}

	sendEvent(ch, index, "start", "")

	taskCtx, cancel := context.WithTimeout(parentCtx, m.taskTimeout)
	defer cancel()

	taskCtx = approval.WithRegistry(taskCtx, approval.NewAutoApproveRegistry())
	taskCtx = copilot.WithAgentInitiator(taskCtx)

	agent, err := m.createSubAgent(taskCtx, fmt.Sprintf("子任务-%d", index), task.Description)
	if err != nil {
		sendEvent(ch, index, "error", fmt.Sprintf("create agent: %v", err))
		return fail(result, statusError, fmt.Sprintf("create agent: %v", err))
	}

	output, ops, err := m.runAgent(taskCtx, agent, index, task.Description, ch)
	if err != nil {
		if taskCtx.Err() != nil {
			if errors.Is(taskCtx.Err(), context.DeadlineExceeded) {
				sendEvent(ch, index, "timeout", "")
				return fail(result, statusTimeout, "task timeout")
			}
			return fail(result, statusError, "cancelled")
		}
		sendEvent(ch, index, "error", err.Error())
		return fail(result, statusError, err.Error())
	}

	sendEvent(ch, index, "done", "")
	result.Status = statusSuccess
	result.Result = output
	result.Operations = ops
	return result
}

func (m *middleware) createSubAgent(ctx context.Context, name, desc string) (*kit.Agent, error) {
	return kit.New(ctx, &kit.Config{
		Name: name, Description: fmt.Sprintf("执行子任务: %s", desc), SystemPrompt: subAgentInstruction,
		Model: m.chatModel, Tools: m.tools, Handlers: m.handlers,
		ModelRetryConfig: kitruntime.AdaptModelRetryConfigForRunner(retry.NewModelRetryConfig()),
		MaxIterations:    retry.MaxIterations(),
	})
}

func (m *middleware) runAgent(ctx context.Context, agent *kit.Agent, index int, desc string, ch chan<- viewEvent) (string, []string, error) {
	defer agent.Close()
	var operations []string
	unsubscribe := agent.Subscribe(func(event kit.Event) {
		switch event.Type {
		case kit.EventMessageDelta:
			sendEvent(ch, index, "content", event.Delta)
		case kit.EventToolStart:
			for _, call := range event.ToolCalls {
				op := fmt.Sprintf("%s(%s)", call.Function.Name, call.Function.Arguments)
				if runes := []rune(op); len(runes) > 120 {
					op = string(runes[:120]) + "..."
				}
				operations = append(operations, op)
				sendEvent(ch, index, "op", op)
			}
		}
	})
	defer unsubscribe()
	result, err := agent.Ask(ctx, desc)
	if err != nil {
		return "", operations, err
	}
	if len(result.Interrupts) > 0 {
		return "", operations, fmt.Errorf("background task requires an interrupt decision")
	}
	return result.Text, operations, nil
}

// sendEvent 安全发送事件到 channel（不会阻塞）
func sendEvent(ch chan<- viewEvent, index int, typ, content string) {
	select {
	case ch <- viewEvent{TaskIndex: index, Type: typ, Content: content}:
	default:
	}
}

func fail(r taskResult, status, errMsg string) taskResult {
	r.Status = status
	r.Error = errMsg
	return r
}

// forwardEvents 在非交互模式下将 dispatch 子任务进度通过 fkevent 转发到父 context
func forwardEvents(ctx context.Context, tasks []taskItem, eventCh <-chan viewEvent) {
	for e := range eventCh {
		desc := ""
		if e.TaskIndex >= 0 && e.TaskIndex < len(tasks) {
			desc = tasks[e.TaskIndex].Description
		}
		detail, _ := json.Marshal(map[string]any{
			"task_index":   e.TaskIndex,
			"description":  desc,
			"event_type":   e.Type,
			"event_detail": e.Content,
		})
		eventType := events.EventSystemNotice
		if e.Type == "start" {
			eventType = events.EventMemberStarted
		}
		if e.Type == "done" || e.Type == "content" {
			eventType = events.EventMemberCompleted
		}
		if e.Type == "error" {
			_ = events.DispatchEvent(ctx, events.Event{
				Type:    events.EventError,
				Content: e.Content,
				Detail:  string(detail),
			})
			continue
		}
		_ = events.DispatchEvent(ctx, events.Event{
			Type:    eventType,
			Content: e.Content,
			Detail:  string(detail),
			Notice: &events.NoticePayload{
				Level:   "info",
				Code:    e.Type,
				Message: e.Content,
			},
		})
	}
}
