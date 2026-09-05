package agentkit

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	domainevent "fkteams/internal/domain/event"
	domainmessage "fkteams/internal/domain/message"
	runtimeport "fkteams/internal/ports/runtime"
	"fkteams/internal/runtime/checkpoint"
	"fkteams/internal/testmodel"
)

func TestRunnerKeepsConcurrentSessionsIsolated(t *testing.T) {
	model := testmodel.New().EnqueueStream(testmodel.AssistantMessage("first")).EnqueueStream(testmodel.AssistantMessage("second"))
	a, err := NewChatModelAgent(context.Background(), &runtimeport.ChatAgentConfig{Name: "shared", Model: model})
	if err != nil {
		t.Fatal(err)
	}
	runner, err := NewRunnerFromConfig(context.Background(), runtimeport.RunnerConfig{Agent: a, EnableStreaming: true})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for _, name := range []string{"session-a", "session-b"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := runner.Run(context.Background(), domainmessage.TurnInput{Message: testmodel.UserMessage(name)}, runtimeport.RunOptions{
				RunID: name,
				Sink: func(e domainevent.Event) error {
					if e.RunID != name {
						t.Errorf("event run = %s, want %s", e.RunID, name)
					}
					if e.Type == domainevent.TypeUserMessage && e.Content != name {
						t.Errorf("history crossed sessions: %s", e.Content)
					}
					return nil
				},
			})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	calls := model.StreamCalls()
	if len(calls) != 2 {
		t.Fatalf("model calls = %d", len(calls))
	}
	for _, call := range calls {
		if len(call.Input) != 1 {
			t.Fatalf("unexpected shared history: %#v", call.Input)
		}
	}
}

func TestRunnerBindsToolsToModel(t *testing.T) {
	tool, err := runtimeport.InferTool("echo", "echo", func(context.Context, *flowEchoRequest) (string, error) { return "ok", nil })
	if err != nil {
		t.Fatal(err)
	}
	model := testmodel.New().EnqueueStream(testmodel.AssistantMessage("done"))
	a, err := NewChatModelAgent(context.Background(), &runtimeport.ChatAgentConfig{Name: "binding", Model: model, Tools: []runtimeport.Tool{tool}})
	if err != nil {
		t.Fatal(err)
	}
	runAgentForTest(t, context.Background(), a, true)
	calls := model.StreamCalls()
	if len(calls) != 1 || len(calls[0].Tools) != 1 || calls[0].Tools[0].Name != "echo" {
		t.Fatalf("tool schema was lost: %#v", calls)
	}
}

func TestRunnerSinkFailureStopsBeforeExecutingTool(t *testing.T) {
	executed := false
	tool, err := runtimeport.InferTool("side_effect", "test", func(context.Context, *flowEchoRequest) (string, error) { executed = true; return "ok", nil })
	if err != nil {
		t.Fatal(err)
	}
	model := testmodel.New().EnqueueStream(toolMessage("call", "side_effect", `{"text":"test"}`))
	a, err := NewChatModelAgent(context.Background(), &runtimeport.ChatAgentConfig{Name: "sink", Model: model, Tools: []runtimeport.Tool{tool}})
	if err != nil {
		t.Fatal(err)
	}
	runner, err := NewRunnerFromConfig(context.Background(), runtimeport.RunnerConfig{Agent: a, EnableStreaming: true})
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("sink closed")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = runner.Run(ctx, domainmessage.TurnInput{Message: testmodel.UserMessage("go")}, runtimeport.RunOptions{Sink: func(e domainevent.Event) error {
		if e.Type == domainevent.TypeToolCallArguments {
			return want
		}
		return nil
	}})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
	if executed {
		t.Fatal("tool executed after sink failure")
	}
}

func TestRunnerRejectsUnhandledInterruptAndCleansCheckpoint(t *testing.T) {
	ctx := runtimeport.WithInterruptRuntime(context.Background(), NewInterruptRuntime())
	var decision any
	gate, err := runtimeport.InferTool("gate", "approval", func(ctx context.Context, _ *flowEchoRequest) (string, error) {
		interrupted, _, _ := runtimeport.GetInterruptState(ctx)
		if !interrupted {
			return "", runtimeport.RequestInterrupt(ctx, "approval required")
		}
		_, _, decision = runtimeport.GetResumeContext[any](ctx)
		return "rejected", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	model := testmodel.New().EnqueueStream(toolMessage("gate-call", "gate", `{"text":"test"}`)).EnqueueStream(testmodel.AssistantMessage("done"))
	a, err := NewChatModelAgent(ctx, &runtimeport.ChatAgentConfig{Name: "approval", Model: model, Tools: []runtimeport.Tool{gate}})
	if err != nil {
		t.Fatal(err)
	}
	store := &trackedStore{MemoryStore: checkpoint.NewMemoryStore().(*checkpoint.MemoryStore)}
	runner, err := NewRunnerFromConfig(ctx, runtimeport.RunnerConfig{Agent: a, EnableStreaming: true, CheckpointStore: store})
	if err != nil {
		t.Fatal(err)
	}
	_, err = runner.Run(ctx, domainmessage.TurnInput{Message: testmodel.UserMessage("go")}, runtimeport.RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if decision != 0 {
		t.Fatalf("default decision = %#v, want rejection", decision)
	}
	for _, key := range store.keys {
		if _, ok, err := store.Get(ctx, key); err != nil || ok {
			t.Fatalf("checkpoint retained: %v %v", ok, err)
		}
	}
}

func TestLoopSharesDiscussionInOrder(t *testing.T) {
	first := testmodel.New().EnqueueStream(testmodel.AssistantMessage("first view"))
	second := testmodel.New().EnqueueStream(testmodel.AssistantMessage("second view"))
	a, err := NewChatModelAgent(context.Background(), &runtimeport.ChatAgentConfig{Name: "first", Model: first})
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewChatModelAgent(context.Background(), &runtimeport.ChatAgentConfig{Name: "second", Model: second})
	if err != nil {
		t.Fatal(err)
	}
	loop, err := NewLoopAgent(context.Background(), &runtimeport.LoopAgentConfig{Name: "discussion", SubAgents: []runtimeport.Agent{a, b}, MaxIterations: 1})
	if err != nil {
		t.Fatal(err)
	}
	runAgentForTest(t, context.Background(), loop, true)
	if !messagesContain(second.StreamCalls()[0].Input, "first view") {
		t.Fatal("second member did not receive first member's response")
	}
}

func toolMessage(id, name, args string) domainmessage.Message {
	return domainmessage.Message{Role: domainmessage.RoleAssistant, ToolCalls: []domainmessage.ToolCall{{ID: id, Index: intPtr(0), Type: "function", Function: domainmessage.FunctionCall{Name: name, Arguments: args}}}}
}

type trackedStore struct {
	*checkpoint.MemoryStore
	keys []string
}

func (s *trackedStore) Set(ctx context.Context, key string, value []byte) error {
	s.keys = append(s.keys, key)
	return s.MemoryStore.Set(ctx, key, value)
}

func TestRunnerCancelsModelRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	model := &cancellingModel{Model: testmodel.New(), started: make(chan struct{})}
	a, err := NewChatModelAgent(ctx, &runtimeport.ChatAgentConfig{Name: "cancel", Model: model})
	if err != nil {
		t.Fatal(err)
	}
	runner, err := NewRunnerFromConfig(ctx, runtimeport.RunnerConfig{Agent: a, EnableStreaming: true})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := runner.Run(ctx, domainmessage.TurnInput{Message: testmodel.UserMessage("go")}, runtimeport.RunOptions{})
		done <- err
	}()
	select {
	case <-model.started:
	case <-time.After(time.Second):
		t.Fatal("model did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancel result = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled runner did not return")
	}
}

type cancellingModel struct {
	*testmodel.Model
	started chan struct{}
}

func (m *cancellingModel) WithTools([]runtimeport.ToolInfo) (runtimeport.ChatModel, error) {
	return m, nil
}
func (m *cancellingModel) Stream(ctx context.Context, _ []domainmessage.Message) (runtimeport.MessageStream, error) {
	close(m.started)
	<-ctx.Done()
	return nil, ctx.Err()
}
