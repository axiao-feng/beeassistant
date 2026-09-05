package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	kit "github.com/wsshow/agentkit"
	"testing"
	"time"

	"fkteams/internal/runtime/events"
)

func TestConfigDefaults(t *testing.T) {
	cfg := Config{}
	cfg.defaults()

	if cfg.MaxConcurrency != defaultMaxConcurrency {
		t.Fatalf("MaxConcurrency = %d, want %d", cfg.MaxConcurrency, defaultMaxConcurrency)
	}
	if cfg.TaskTimeout != defaultTaskTimeout {
		t.Fatalf("TaskTimeout = %s, want %s", cfg.TaskTimeout, defaultTaskTimeout)
	}

	cfg = Config{MaxConcurrency: 8, TaskTimeout: time.Second}
	cfg.defaults()
	if cfg.MaxConcurrency != 8 || cfg.TaskTimeout != time.Second {
		t.Fatalf("defaults overwrote configured values: concurrency=%d timeout=%s", cfg.MaxConcurrency, cfg.TaskTimeout)
	}
}

func TestExecuteTasksEmptyInput(t *testing.T) {
	m := &middleware{maxConcurrency: 1, taskTimeout: time.Second}

	got, err := m.executeTasks(context.Background(), &dispatchInput{})
	if err != nil {
		t.Fatalf("executeTasks returned error: %v", err)
	}
	if got != `{"results":[]}` {
		t.Fatalf("executeTasks empty result = %q", got)
	}
}

func TestFailSetsStatusAndError(t *testing.T) {
	got := fail(taskResult{TaskIndex: 1, Description: "任务"}, statusError, "boom")
	if got.Status != statusError || got.Error != "boom" || got.TaskIndex != 1 || got.Description != "任务" {
		t.Fatalf("fail result = %#v", got)
	}
}

func TestSendEventDropsWhenChannelIsFull(t *testing.T) {
	ch := make(chan viewEvent, 1)
	sendEvent(ch, 0, "start", "")
	sendEvent(ch, 1, "done", "ignored")

	got := <-ch
	if got.TaskIndex != 0 || got.Type != "start" {
		t.Fatalf("first event = %#v, want start event", got)
	}
	select {
	case extra := <-ch:
		t.Fatalf("sendEvent should drop when full, got %#v", extra)
	default:
	}
}

func TestForwardEventsDispatchesMemberUpdates(t *testing.T) {
	ch := make(chan viewEvent, 3)
	ch <- viewEvent{TaskIndex: 0, Type: "start"}
	ch <- viewEvent{TaskIndex: 1, Type: "content", Content: "完成"}
	ch <- viewEvent{TaskIndex: 9, Type: "error", Content: "bad index"}
	close(ch)

	var got []events.Event
	ctx := events.WithCallback(events.WithNonInteractive(context.Background()), func(event events.Event) error {
		got = append(got, event)
		return nil
	})
	tasks := []taskItem{{Description: "第一项"}, {Description: "第二项"}}
	forwardEvents(ctx, tasks, ch)

	if len(got) != 3 {
		t.Fatalf("forwarded events count = %d, want 3", len(got))
	}
	if got[0].Type != events.EventMemberStarted {
		t.Fatalf("first forwarded event = %#v", got[0])
	}
	if got[1].Content != "完成" || got[1].Type != events.EventMemberCompleted {
		t.Fatalf("second forwarded event = %#v", got[1])
	}

	var detail struct {
		TaskIndex   int    `json:"task_index"`
		Description string `json:"description"`
		EventType   string `json:"event_type"`
		EventDetail string `json:"event_detail"`
	}
	if err := json.Unmarshal([]byte(got[1].Detail), &detail); err != nil {
		t.Fatalf("unmarshal detail: %v", err)
	}
	if detail.TaskIndex != 1 || detail.Description != "第二项" || detail.EventType != "content" || detail.EventDetail != "完成" {
		t.Fatalf("detail = %#v", detail)
	}

	if err := json.Unmarshal([]byte(got[2].Detail), &detail); err != nil {
		t.Fatalf("unmarshal out of range detail: %v", err)
	}
	if detail.Description != "" || detail.TaskIndex != 9 {
		t.Fatalf("out of range detail = %#v", detail)
	}
}

func TestRunAgentReturnsFinalTextAndToolOperations(t *testing.T) {
	tool, err := kit.NewMockTool("echo", "echo", func(context.Context, *struct {
		Text string `json:"text"`
	}) (string, error) {
		return "echoed", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	call := tool.Call("echo-call", &struct {
		Text string `json:"text"`
	}{Text: "hello"})
	model := kit.NewMockChatModel(kit.MockModelCalls(call), kit.MockModelStream("final ", "answer"))
	a, err := kit.New(context.Background(), &kit.Config{Name: "dispatch", Model: model, Tools: kit.MockTools(tool)})
	if err != nil {
		t.Fatal(err)
	}
	ch := make(chan viewEvent, 16)
	text, ops, err := (&middleware{}).runAgent(context.Background(), a, 0, "go", ch)
	if err != nil {
		t.Fatal(err)
	}
	if text != "final answer" || len(ops) != 1 {
		t.Fatalf("result = %q, %#v", text, ops)
	}
}

func TestRunAgentPropagatesStreamFailure(t *testing.T) {
	want := errors.New("stream failed")
	a, err := kit.New(context.Background(), &kit.Config{Name: "dispatch", Model: kit.NewMockChatModel(kit.MockModelStreamError(want, "partial"))})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = (&middleware{}).runAgent(context.Background(), a, 0, "go", make(chan viewEvent, 16))
	if !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
}
