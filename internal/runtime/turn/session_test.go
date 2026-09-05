package turn

import (
	"context"
	"fkteams/internal/domain/message"
	runtimeport "fkteams/internal/ports/runtime"
	"fkteams/internal/runtime/approval"
	"fkteams/internal/runtime/events"
	"fkteams/internal/runtime/hooks"
	"testing"
)

type summarySinkStub struct {
	count int
}

func (h *summarySinkStub) GetMessageCount() int {
	return h.count
}

func (h *summarySinkStub) SetSummary(string, int) {}

type runnerStub struct {
	input message.TurnInput
	opts  runtimeport.RunOptions
}

func (r *runnerStub) Run(_ context.Context, input message.TurnInput, opts runtimeport.RunOptions) (*runtimeport.RunResult, error) {
	r.input = input
	r.opts = opts
	return &runtimeport.RunResult{}, nil
}

func TestExecutorRunUsesConfiguredRunID(t *testing.T) {
	runner := &runnerStub{}
	_, err := NewExecutor().Run(context.Background(), Request{
		Runner:    runner,
		SessionID: "session-1",
		RunID:     "run-1",
		Input:     TurnInput{Message: message.Message{Role: message.RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if runner.opts.RunID != "run-1" {
		t.Fatalf("run id = %q, want run-1", runner.opts.RunID)
	}
}

func TestExecutorRunConfiguresRequestCapabilities(t *testing.T) {
	history := &summarySinkStub{}
	approvalReg := approval.NewDefaultRegistry()
	eventHandler := func(events.Event) error { return nil }
	startHandler := func(context.Context) {}
	interruptHandler := FixedDecisionHandler(approval.Reject)
	finishHandler := func(context.Context, *runtimeport.RunResult, error) {}
	bus := hooks.NewBus()

	runner := &runnerStub{}
	_, err := NewExecutor().Run(context.Background(), Request{
		Runner:         runner,
		SessionID:      "session-1",
		Input:          TurnInput{Context: []message.Message{{Role: message.RoleSystem, Content: "context"}}},
		EventSink:      eventHandler,
		Summary:        history,
		OnStart:        startHandler,
		OnInterrupt:    interruptHandler,
		NonInteractive: true,
		ContextHooks: []ContextHook{
			approval.RegistryContext(approvalReg),
		},
		HookBus:  bus,
		OnFinish: finishHandler,
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if len(runner.input.Context) != 1 || runner.input.Context[0].Content != "context" {
		t.Fatalf("input context = %#v, want configured context", runner.input.Context)
	}
}

func TestExecutorRunRejectsMissingDependencies(t *testing.T) {
	if _, err := NewExecutor().Run(context.Background(), Request{SessionID: "s"}); err == nil {
		t.Fatal("expected missing runner error")
	}
	if _, err := NewExecutor().Run(context.Background(), Request{Runner: &runnerStub{}}); err == nil {
		t.Fatal("expected missing session ID error")
	}
}
