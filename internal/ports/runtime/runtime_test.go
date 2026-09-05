package runtime

import (
	"context"
	"testing"

	"fkteams/internal/domain/event"
	"fkteams/internal/domain/message"
)

func TestRunOptionsWithDefaults(t *testing.T) {
	opts := RunOptions{RunID: "run-1"}.WithDefaults("default-run")

	if opts.RunID != "run-1" {
		t.Fatalf("run id = %q, want run-1", opts.RunID)
	}
	if opts.Sink == nil {
		t.Fatal("sink was not defaulted")
	}
	if err := opts.Sink(event.Event{}); err != nil {
		t.Fatalf("default sink returned error: %v", err)
	}
}

func TestRunOptionsWithDefaultsUsesFallbackRunID(t *testing.T) {
	opts := RunOptions{}.WithDefaults("default-run")

	if opts.RunID != "default-run" {
		t.Fatalf("run id = %q, want default-run", opts.RunID)
	}
}

func TestSummaryPersistCallbackContext(t *testing.T) {
	called := false
	cb := SummaryPersistCallback(func(summary string) {
		called = summary == "done"
	})

	ctx := WithSummaryPersistCallback(context.Background(), cb)
	got, ok := SummaryPersistCallbackFromContext(ctx)
	if !ok {
		t.Fatal("expected summary callback in context")
	}
	got("done")
	if !called {
		t.Fatal("summary callback was not invoked")
	}

	if _, ok := SummaryPersistCallbackFromContext(context.Background()); ok {
		t.Fatal("plain context should not contain summary callback")
	}
}

func TestSteeringSourceContext(t *testing.T) {
	ctx := WithSteeringSource(context.Background(), nil)
	if _, ok := SteeringSourceFromContext(ctx); ok {
		t.Fatal("nil steering source should not be stored")
	}

	source := SteeringSource(func(context.Context) ([]message.Message, error) {
		return []message.Message{{Role: message.RoleUser, Content: "steer"}}, nil
	})
	ctx = WithSteeringSource(context.Background(), source)
	got, ok := SteeringSourceFromContext(ctx)
	if !ok {
		t.Fatal("expected steering source in context")
	}
	messages, err := got(context.Background())
	if err != nil {
		t.Fatalf("steering source returned error: %v", err)
	}
	if len(messages) != 1 || messages[0].Content != "steer" {
		t.Fatalf("steering messages = %#v", messages)
	}
}
