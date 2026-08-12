package hooks

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"fkteams/internal/domain/message"
	"fkteams/internal/domain/session"
)

func TestBusInvokesHandlersInPriorityOrder(t *testing.T) {
	bus := NewBus()
	var got []string
	bus.RegisterFunc("second", []HookPoint{HookBeforeRun}, func(ctx Context, inv Invocation) (Result, error) {
		got = append(got, "second")
		return Result{}, nil
	}, Options{Priority: 20})
	bus.RegisterFunc("first", []HookPoint{HookBeforeRun}, func(ctx Context, inv Invocation) (Result, error) {
		got = append(got, "first")
		return Result{}, nil
	}, Options{Priority: 10})

	if _, err := bus.Invoke(context.Background(), Invocation{HookPoint: HookBeforeRun}); err != nil {
		t.Fatalf("invoke hooks: %v", err)
	}
	if len(got) != 2 || got[0] != "first" || got[1] != "second" {
		t.Fatalf("order = %#v, want first, second", got)
	}
}

func TestBusPassesMutatedPayload(t *testing.T) {
	bus := NewBus()
	bus.RegisterFunc("mutate", []HookPoint{HookBeforeRun}, func(ctx Context, inv Invocation) (Result, error) {
		payload := inv.Payload.(BeforeRunPayload)
		payload.Input.Message.Content = "changed"
		return Result{Payload: payload}, nil
	}, Options{})

	result, err := bus.Invoke(context.Background(), Invocation{
		HookPoint: HookBeforeRun,
		Payload: BeforeRunPayload{Input: message.TurnInput{
			Message: message.Message{Role: message.RoleUser, Content: "original"},
		}},
	})
	if err != nil {
		t.Fatalf("invoke hooks: %v", err)
	}
	payload, ok := result.Payload.(BeforeRunPayload)
	if !ok || payload.Input.Message.Content != "changed" {
		t.Fatalf("payload = %#v, want changed before-run payload", result.Payload)
	}
}

func TestBusRejectsMismatchedPayloadPoint(t *testing.T) {
	bus := NewBus()
	_, err := bus.Invoke(context.Background(), Invocation{
		HookPoint: HookBeforeRun,
		Payload:   EventPayload{},
	})
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("error = %v, want payload mismatch", err)
	}
}

func TestBusAddsSessionIDFromContext(t *testing.T) {
	bus := NewBus()
	var got string
	bus.RegisterFunc("capture", []HookPoint{HookBeforeRun}, func(_ Context, inv Invocation) (Result, error) {
		got = inv.SessionID
		return Result{}, nil
	}, Options{})
	ctx := session.WithID(context.Background(), "session-1")
	if _, err := bus.Invoke(ctx, Invocation{HookPoint: HookBeforeRun}); err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if got != "session-1" {
		t.Fatalf("session id = %q", got)
	}
}

func TestBusConcurrentHandlerAllowsParallelCalls(t *testing.T) {
	bus := NewBus()
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	bus.RegisterFunc("parallel", []HookPoint{HookBeforeRun}, func(_ Context, _ Invocation) (Result, error) {
		started <- struct{}{}
		<-release
		return Result{}, nil
	}, Options{Concurrent: true, Timeout: time.Second})
	done := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			_, err := bus.Invoke(context.Background(), Invocation{HookPoint: HookBeforeRun})
			done <- err
		}()
	}
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(100 * time.Millisecond):
			t.Fatal("concurrent handler calls were serialized")
		}
	}
	close(release)
	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			t.Fatalf("Invoke() error = %v", err)
		}
	}
}

func TestBusWarnPolicyContinuesAfterError(t *testing.T) {
	bus := NewBus()
	called := false
	bus.RegisterFunc("bad", []HookPoint{HookOnEvent}, func(ctx Context, inv Invocation) (Result, error) {
		return Result{}, errors.New("boom")
	}, Options{ErrorPolicy: ErrorWarn})
	bus.RegisterFunc("next", []HookPoint{HookOnEvent}, func(ctx Context, inv Invocation) (Result, error) {
		called = true
		return Result{}, nil
	}, Options{})

	if _, err := bus.Invoke(context.Background(), Invocation{HookPoint: HookOnEvent}); err != nil {
		t.Fatalf("invoke hooks: %v", err)
	}
	if !called {
		t.Fatal("next hook was not called")
	}
}

func TestBusFailPolicyReturnsError(t *testing.T) {
	bus := NewBus()
	bus.RegisterFunc("bad", []HookPoint{HookBeforeRun}, func(ctx Context, inv Invocation) (Result, error) {
		return Result{}, errors.New("boom")
	}, Options{})

	if _, err := bus.Invoke(context.Background(), Invocation{HookPoint: HookBeforeRun}); err == nil {
		t.Fatal("expected error")
	}
}

func TestBusTimesOutHandler(t *testing.T) {
	bus := NewBus()
	bus.RegisterFunc("slow", []HookPoint{HookBeforeRun}, func(ctx Context, inv Invocation) (Result, error) {
		time.Sleep(time.Second)
		return Result{}, nil
	}, Options{Timeout: time.Millisecond})

	if _, err := bus.Invoke(context.Background(), Invocation{HookPoint: HookBeforeRun}); err == nil {
		t.Fatal("expected timeout")
	}
}

func TestBusDoesNotSpawnRepeatedStuckHandlerCalls(t *testing.T) {
	bus := NewBus()
	blocked := make(chan struct{})
	var calls atomic.Int32
	bus.RegisterFunc("stuck", []HookPoint{HookBeforeRun}, func(ctx Context, inv Invocation) (Result, error) {
		calls.Add(1)
		<-blocked
		return Result{}, nil
	}, Options{Timeout: 5 * time.Millisecond})

	for i := 0; i < 5; i++ {
		if _, err := bus.Invoke(context.Background(), Invocation{HookPoint: HookBeforeRun}); err == nil {
			t.Fatalf("invoke %d should time out", i)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("stuck handler call count = %d, want 1", got)
	}
	close(blocked)
}

func TestNilBusIsNoop(t *testing.T) {
	var bus *Bus
	input := message.TurnInput{Message: message.Message{Role: message.RoleUser, Content: "ping"}}

	got, err := bus.InvokeBeforeRun(context.Background(), input)
	if err != nil {
		t.Fatalf("nil bus before run: %v", err)
	}
	if got.Message.Content != "ping" {
		t.Fatalf("input = %#v, want unchanged", got)
	}
}
